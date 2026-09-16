package promoter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

func (r PostgresRepository) ExecuteAdmin(ctx context.Context, actorID string, c AdminCommand) (Profile, error) {
	if c.Action != ReviewAction && c.Action != DisableAction {
		return Profile{}, ErrInvalidInput
	}
	request, err := json.Marshal(c)
	if err != nil {
		return Profile{}, err
	}
	sum := sha256.Sum256(request)
	fingerprint := hex.EncodeToString(sum[:])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()
	// Serialize the actor/action/key scope even when two requests name different targets.
	lockScope, _ := json.Marshal([]string{actorID, c.Action, c.IdempotencyKey})
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, string(lockScope)); err != nil {
		return Profile{}, err
	}
	var previousFingerprint string
	var snapshot []byte
	err = tx.QueryRowContext(ctx, `SELECT request_fingerprint,after_profile FROM promoter_admin_audits WHERE actor_id=$1 AND action=$2 AND idempotency_key=$3`, actorID, c.Action, c.IdempotencyKey).Scan(&previousFingerprint, &snapshot)
	if err == nil {
		if previousFingerprint != fingerprint {
			return Profile{}, ErrIdempotencyConflict
		}
		var previous Profile
		if err = json.Unmarshal(snapshot, &previous); err != nil {
			return Profile{}, err
		}
		if err = tx.Commit(); err != nil {
			return Profile{}, err
		}
		return previous, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Profile{}, err
	}
	userID := c.TargetID
	if c.Action == ReviewAction {
		err = tx.QueryRowContext(ctx, `SELECT user_id FROM promoter_applications WHERE id=$1`, c.TargetID).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		if err != nil {
			return Profile{}, err
		}
	}
	// Even an administrator cannot approve or disable their own membership.
	if actorID == userID {
		return Profile{}, ErrForbidden
	}
	var before Profile
	err = tx.QueryRowContext(ctx, `SELECT user_id,status,reason,COALESCE(application_id,''),version FROM promoter_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&before.UserID, &before.Status, &before.Reason, &before.ApplicationID, &before.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	var after Profile
	if c.Action == ReviewAction {
		if before.ApplicationID != c.TargetID {
			return Profile{}, ErrConflict
		}
		after, err = before.Review(c.Approve, c.Reason, c.Version)
	} else {
		after, err = before.Disable(c.Reason, c.Version)
	}
	if err != nil {
		return Profile{}, err
	}
	if c.Action == ReviewAction {
		result, err := tx.ExecContext(ctx, `UPDATE promoter_applications SET status=$2 WHERE id=$1 AND status='PENDING'`, c.TargetID, after.Status)
		if err != nil {
			return Profile{}, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return Profile{}, err
		}
		if rows != 1 {
			return Profile{}, ErrConflict
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE promoter_profiles SET status=$2,reason=$3,version=$4 WHERE user_id=$1`, userID, after.Status, after.Reason, after.Version)
	if err != nil {
		return Profile{}, err
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return Profile{}, err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return Profile{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO promoter_admin_audits(actor_id,action,idempotency_key,user_id,request_fingerprint,before_profile,after_profile) VALUES($1,$2,$3,$4,$5,$6,$7)`, actorID, c.Action, c.IdempotencyKey, userID, fingerprint, string(beforeJSON), string(afterJSON))
	if err != nil {
		return Profile{}, err
	}
	if err = tx.Commit(); err != nil {
		return Profile{}, err
	}
	return after, nil
}
