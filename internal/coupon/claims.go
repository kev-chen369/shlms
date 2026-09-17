package coupon

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrClaimInvalid         = errors.New("invalid claim")
	ErrClaimNotFound        = errors.New("claim not found")
	ErrClaimConflict        = errors.New("claim idempotency conflict")
	claimFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Claim struct {
	ID                 string    `json:"claimId"`
	OwnerUserID        string    `json:"-"`
	CouponID           string    `json:"couponId"`
	IdempotencyKey     string    `json:"-"`
	RequestFingerprint string    `json:"-"`
	Status             string    `json:"status"`
	EvidenceRef        string    `json:"-"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type ClaimStore struct{ DB *sql.DB }

const claimColumns = `id,owner_user_id,coupon_id,idempotency_key,request_fingerprint,status,evidence_ref,created_at,updated_at`

func scanClaim(row *sql.Row) (Claim, error) {
	var c Claim
	err := row.Scan(&c.ID, &c.OwnerUserID, &c.CouponID, &c.IdempotencyKey, &c.RequestFingerprint,
		&c.Status, &c.EvidenceRef, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Claim{}, ErrClaimNotFound
	}
	return c, err
}

func (s ClaimStore) Get(ctx context.Context, owner, id string) (Claim, error) {
	if s.DB == nil || !validID(owner) || !validID(id) {
		return Claim{}, ErrClaimInvalid
	}
	return scanClaim(s.DB.QueryRowContext(ctx, `SELECT `+claimColumns+` FROM coupon_claims WHERE owner_user_id=$1 AND id=$2`, owner, id))
}

func (s ClaimStore) GetByKey(ctx context.Context, owner, key string) (Claim, error) {
	if s.DB == nil || !validID(owner) || !validClaimKey(key) {
		return Claim{}, ErrClaimInvalid
	}
	return scanClaim(s.DB.QueryRowContext(ctx, `SELECT `+claimColumns+` FROM coupon_claims WHERE owner_user_id=$1 AND idempotency_key=$2`, owner, key))
}

// Reserve creates one logical request. Same owner/key/fingerprint replays the
// original row; a changed request never reuses the key or invokes the channel.
func (s ClaimStore) Reserve(ctx context.Context, c Claim) (Claim, bool, error) {
	if s.DB == nil || !validID(c.ID) || !validID(c.OwnerUserID) || !validID(c.CouponID) ||
		!validClaimKey(c.IdempotencyKey) || !claimFingerprintPattern.MatchString(c.RequestFingerprint) {
		return Claim{}, false, ErrClaimInvalid
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO coupon_claims
		(id,owner_user_id,coupon_id,idempotency_key,request_fingerprint,status)
		VALUES($1,$2,$3,$4,$5,'PENDING') ON CONFLICT(owner_user_id,idempotency_key) DO NOTHING`,
		c.ID, c.OwnerUserID, c.CouponID, c.IdempotencyKey, c.RequestFingerprint)
	if err != nil {
		return Claim{}, false, err
	}
	created, err := result.RowsAffected()
	if err != nil {
		return Claim{}, false, err
	}
	stored, err := scanClaim(s.DB.QueryRowContext(ctx, `SELECT `+claimColumns+` FROM coupon_claims WHERE owner_user_id=$1 AND idempotency_key=$2`, c.OwnerUserID, c.IdempotencyKey))
	if err != nil {
		return Claim{}, false, err
	}
	if stored.RequestFingerprint != c.RequestFingerprint || stored.CouponID != c.CouponID {
		return Claim{}, false, ErrClaimConflict
	}
	return stored, created == 1, nil
}

func validClaimKey(key string) bool {
	if len(key) == 0 || len(key) > 128 {
		return false
	}
	for _, b := range []byte(key) {
		if b < 33 || b > 126 {
			return false
		}
	}
	return true
}

// SetOutcome cannot turn an unknown or failed result into a success without a
// separate trusted channel receipt. Terminal statuses are immutable.
func (s ClaimStore) SetOutcome(ctx context.Context, owner, id, status, evidence string) (Claim, error) {
	if s.DB == nil || !validID(owner) || !validID(id) ||
		(status != "QUERY_REQUIRED" && status != "CLAIMED" && status != "FAILED") ||
		(status == "CLAIMED" && (!validText(evidence, 256) || strings.Contains(evidence, "://"))) ||
		(status != "CLAIMED" && evidence != "") {
		return Claim{}, ErrClaimInvalid
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE coupon_claims SET status=$3,evidence_ref=$4,updated_at=CURRENT_TIMESTAMP
		WHERE owner_user_id=$1 AND id=$2 AND status IN ('PENDING','QUERY_REQUIRED')`, owner, id, status, evidence)
	if err != nil {
		return Claim{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Claim{}, err
	}
	current, err := s.Get(ctx, owner, id)
	if err != nil {
		return Claim{}, err
	}
	if changed == 0 && (current.Status != status || current.EvidenceRef != evidence) {
		return Claim{}, ErrClaimConflict
	}
	return current, nil
}
