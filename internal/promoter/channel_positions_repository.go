package promoter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func scanChannelPosition(row rowScanner) (ChannelPosition, error) {
	var p ChannelPosition
	err := row.Scan(&p.PositionID, &p.Channel, &p.AccountID, &p.ExternalPositionID, &p.Status, &p.Version, &p.ConfiguredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelPosition{}, ErrNotFound
	}
	return p, err
}

func (r PostgresRepository) ConfigureChannelPosition(ctx context.Context, actorID string, in ConfigureChannelPositionInput) (ChannelPosition, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return ChannelPosition{}, err
	}
	sum := sha256.Sum256(b)
	fingerprint := hex.EncodeToString(sum[:])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ChannelPosition{}, err
	}
	defer tx.Rollback()
	// The actor/key lock also protects reuse of the key for a different position.
	lockScope, _ := json.Marshal([]string{actorID, in.IdempotencyKey})
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, string(lockScope)); err != nil {
		return ChannelPosition{}, err
	}
	var previousFingerprint string
	var receipt []byte
	err = tx.QueryRowContext(ctx, `SELECT request_fingerprint,after_config FROM channel_position_config_events WHERE actor_id=$1 AND idempotency_key=$2`, actorID, in.IdempotencyKey).Scan(&previousFingerprint, &receipt)
	if err == nil {
		if previousFingerprint != fingerprint {
			return ChannelPosition{}, ErrIdempotencyConflict
		}
		var previous ChannelPosition
		if err = json.Unmarshal(receipt, &previous); err != nil {
			return ChannelPosition{}, err
		}
		if err = tx.Commit(); err != nil {
			return ChannelPosition{}, err
		}
		return previous, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ChannelPosition{}, err
	}
	var ownerID string
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id FROM promotion_positions WHERE id=$1`, in.PositionID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelPosition{}, ErrNotFound
	}
	if err != nil {
		return ChannelPosition{}, err
	}
	// Match the lock order used by user position changes and membership review.
	var membership Status
	err = tx.QueryRowContext(ctx, `SELECT status FROM promoter_profiles WHERE user_id=$1 FOR UPDATE`, ownerID).Scan(&membership)
	if err != nil {
		return ChannelPosition{}, err
	}
	if membership != Enabled {
		return ChannelPosition{}, ErrNotEnabled
	}
	var positionStatus Status
	err = tx.QueryRowContext(ctx, `SELECT status FROM promotion_positions WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, in.PositionID, ownerID).Scan(&positionStatus)
	if err != nil {
		return ChannelPosition{}, err
	}
	if positionStatus != Enabled {
		return ChannelPosition{}, ErrTransition
	}
	var before ChannelPosition
	before, err = scanChannelPosition(tx.QueryRowContext(ctx, `SELECT position_id,channel,account_id,external_position_id,status,version,configured_at FROM channel_positions WHERE position_id=$1 AND channel=$2 FOR UPDATE`, in.PositionID, in.Channel))
	if errors.Is(err, ErrNotFound) {
		before = ChannelPosition{}
	} else if err != nil {
		return ChannelPosition{}, err
	}
	if before.Version != in.ExpectedVersion {
		return ChannelPosition{}, ErrConflict
	}
	var after ChannelPosition
	if before.Version == 0 {
		after, err = scanChannelPosition(tx.QueryRowContext(ctx, `INSERT INTO channel_positions(position_id,channel,account_id,external_position_id,version) VALUES($1,$2,$3,$4,1)
			RETURNING position_id,channel,account_id,external_position_id,status,version,configured_at`, in.PositionID, in.Channel, in.AccountID, in.ExternalPositionID))
	} else {
		after, err = scanChannelPosition(tx.QueryRowContext(ctx, `UPDATE channel_positions SET account_id=$3,external_position_id=$4,status='PENDING_VERIFICATION',version=version+1,configured_at=CURRENT_TIMESTAMP WHERE position_id=$1 AND channel=$2
			RETURNING position_id,channel,account_id,external_position_id,status,version,configured_at`, in.PositionID, in.Channel, in.AccountID, in.ExternalPositionID))
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "channel_positions_channel_account_id_external_position_id_key" {
			return ChannelPosition{}, ErrExternalPositionConflict
		}
		return ChannelPosition{}, err
	}
	var beforeJSON any
	if before.Version > 0 {
		v, e := json.Marshal(before)
		if e != nil {
			return ChannelPosition{}, e
		}
		beforeJSON = string(v)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return ChannelPosition{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO channel_position_config_events(actor_id,idempotency_key,position_id,request_fingerprint,before_config,after_config) VALUES($1,$2,$3,$4,$5,$6)`, actorID, in.IdempotencyKey, in.PositionID, fingerprint, beforeJSON, string(afterJSON))
	if err != nil {
		return ChannelPosition{}, err
	}
	if err = tx.Commit(); err != nil {
		return ChannelPosition{}, err
	}
	return after, nil
}
