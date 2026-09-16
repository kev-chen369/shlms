package preview

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalid             = errors.New("invalid preview")
	ErrNotFound            = errors.New("preview not found")
	ErrIdempotencyConflict = errors.New("preview idempotency conflict")
	fingerprintPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Snapshot contains only an authorized channel quote. It is not a payable balance.
// EvidenceRef must be an opaque non-secret reference to upstream evidence, never a URL or token.
type Snapshot struct {
	ID                            string
	OwnerUserID                   string
	PositionID                    string
	IdempotencyKey                string
	RequestFingerprint            string
	Scene                         string
	Channel                       string
	ExternalProductID             string
	ProductName                   string
	Currency                      string
	CouponPriceMinor              int64
	PromoterEstimateMinor         int64
	ConsumerCashbackEstimateMinor int64
	RuleVersion                   string
	EvidenceRef                   string
	UpdatedAt                     time.Time
	ExpiresAt                     time.Time
}

func (s Snapshot) valid() bool {
	for _, v := range []struct {
		value string
		max   int
	}{
		{s.ID, 128}, {s.OwnerUserID, 128}, {s.PositionID, 128},
		{s.IdempotencyKey, 128}, {s.Scene, 80}, {s.ExternalProductID, 128},
		{s.ProductName, 256}, {s.RuleVersion, 80}, {s.EvidenceRef, 256},
	} {
		if v.value != strings.TrimSpace(v.value) || len(v.value) == 0 || len(v.value) > v.max {
			return false
		}
	}
	return s.Channel == "JD" && s.Currency == "CNY" &&
		fingerprintPattern.MatchString(s.RequestFingerprint) &&
		s.CouponPriceMinor >= 0 && s.PromoterEstimateMinor >= 0 &&
		s.ConsumerCashbackEstimateMinor >= 0 && !s.UpdatedAt.IsZero() &&
		s.ExpiresAt.After(s.UpdatedAt) && s.ExpiresAt.Sub(s.UpdatedAt) <= 24*time.Hour
}

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) Repository { return Repository{db: db} }

const columns = `id,owner_user_id,position_id,idempotency_key,request_fingerprint,scene,channel,external_product_id,product_name,currency,coupon_price_minor,promoter_estimate_minor,consumer_cashback_estimate_minor,rule_version,evidence_ref,updated_at,expires_at`

func scan(row *sql.Row) (Snapshot, error) {
	var s Snapshot
	err := row.Scan(&s.ID, &s.OwnerUserID, &s.PositionID, &s.IdempotencyKey, &s.RequestFingerprint,
		&s.Scene, &s.Channel, &s.ExternalProductID, &s.ProductName, &s.Currency,
		&s.CouponPriceMinor, &s.PromoterEstimateMinor, &s.ConsumerCashbackEstimateMinor,
		&s.RuleVersion, &s.EvidenceRef, &s.UpdatedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, ErrNotFound
	}
	return s, err
}

// Save is retry-safe for one owner and key. A different request fingerprint never replaces a quote.
// The caller must validate promoter state, readiness and upstream evidence before invoking Save.
func (r Repository) Save(ctx context.Context, s Snapshot) (Snapshot, error) {
	if !s.valid() {
		return Snapshot{}, ErrInvalid
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO promotion_previews (`+columns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (owner_user_id,idempotency_key) DO NOTHING`, s.ID, s.OwnerUserID,
		s.PositionID, s.IdempotencyKey, s.RequestFingerprint, s.Scene, s.Channel,
		s.ExternalProductID, s.ProductName, s.Currency, s.CouponPriceMinor,
		s.PromoterEstimateMinor, s.ConsumerCashbackEstimateMinor, s.RuleVersion,
		s.EvidenceRef, s.UpdatedAt, s.ExpiresAt)
	if err != nil {
		return Snapshot{}, err
	}
	stored, err := scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM promotion_previews WHERE owner_user_id=$1 AND idempotency_key=$2`, s.OwnerUserID, s.IdempotencyKey))
	if err != nil {
		return Snapshot{}, err
	}
	if stored.RequestFingerprint != s.RequestFingerprint {
		return Snapshot{}, ErrIdempotencyConflict
	}
	return stored, nil
}

// FindByID deliberately requires the authenticated owner, including for expired history.
func (r Repository) FindByID(ctx context.Context, ownerUserID, id string) (Snapshot, error) {
	return scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM promotion_previews WHERE owner_user_id=$1 AND id=$2`, ownerUserID, id))
}
