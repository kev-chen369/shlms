package promoter

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrIdempotencyConflict = errors.New("application idempotency conflict")

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) PostgresRepository { return PostgresRepository{db: db} }

func (r PostgresRepository) FindByUserID(ctx context.Context, userID string) (Profile, error) {
	var p Profile
	err := r.db.QueryRowContext(ctx, `SELECT user_id,status,reason,COALESCE(application_id,''),version FROM promoter_profiles WHERE user_id=$1`, userID).Scan(&p.UserID, &p.Status, &p.Reason, &p.ApplicationID, &p.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}

// Submit atomically serializes submissions per user, including first-time
// applicants. Retries return the immutable original submission, even after review.
// Admin review must lock the same profile and update both statuses in one transaction.
func (r PostgresRepository) Submit(ctx context.Context, key string, a Application) (Application, error) {
	if strings.TrimSpace(key) == "" || len(key) > 128 {
		return Application{}, ErrInvalidInput
	}
	if _, err := (Profile{UserID: a.UserID, Status: NotApplied}).Apply(a, 0); err != nil {
		return Application{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Application{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED') ON CONFLICT(user_id) DO NOTHING`, a.UserID); err != nil {
		return Application{}, err
	}
	var p Profile
	err = tx.QueryRowContext(ctx, `SELECT user_id,status,reason,COALESCE(application_id,''),version FROM promoter_profiles WHERE user_id=$1 FOR UPDATE`, a.UserID).Scan(&p.UserID, &p.Status, &p.Reason, &p.ApplicationID, &p.Version)
	if err != nil {
		return Application{}, err
	}
	var old Application
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,display_name,scene,agreement_version,consented_at FROM promoter_applications WHERE user_id=$1 AND idempotency_key=$2`, a.UserID, key).Scan(&old.ID, &old.UserID, &old.DisplayName, &old.Scene, &old.AgreementVersion, &old.ConsentedAt)
	if err == nil {
		if old.DisplayName != a.DisplayName || old.Scene != a.Scene || old.AgreementVersion != a.AgreementVersion {
			return Application{}, ErrIdempotencyConflict
		}
		if err = tx.Commit(); err != nil {
			return Application{}, err
		}
		return old, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Application{}, err
	}
	next, err := p.Apply(a, p.Version)
	if err != nil {
		return Application{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status) VALUES($1,$2,$3,$4,$5,$6,$7,'PENDING')`, a.ID, a.UserID, key, a.DisplayName, a.Scene, a.AgreementVersion, a.ConsentedAt)
	if err != nil {
		return Application{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE promoter_profiles SET status=$2,reason='',application_id=$3,version=$4 WHERE user_id=$1`, next.UserID, next.Status, next.ApplicationID, next.Version)
	if err != nil {
		return Application{}, err
	}
	if err = tx.Commit(); err != nil {
		return Application{}, err
	}
	return a, nil
}
