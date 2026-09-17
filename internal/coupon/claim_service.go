package coupon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrClaimUnavailable = errors.New("coupon claim channel is unavailable")
	ErrNotClaimable     = errors.New("coupon cannot be claimed in this app")
)

type ClaimInput struct {
	OwnerUserID    string
	CouponID       string
	CityCode       string
	Business       string
	IdempotencyKey string
}

type Outcome struct {
	Status      string
	EvidenceRef string
}

// ClaimAdapter must use the stable claim ID as its upstream request reference.
// Query checks that reference and must never create a second claim.
type ClaimAdapter interface {
	Claim(context.Context, Claim) (Outcome, error)
	Query(context.Context, Claim) (Outcome, error)
}

type ClaimService struct {
	Catalog Catalog
	Store   ClaimStore
	Adapter ClaimAdapter
}

func claimFingerprint(in ClaimInput) string {
	sum := sha256.Sum256([]byte(in.CouponID + "\x00" + in.CityCode + "\x00" + in.Business))
	return hex.EncodeToString(sum[:])
}

func newClaimID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func (s ClaimService) Create(ctx context.Context, in ClaimInput) (Claim, error) {
	if s.Adapter == nil || s.Store.DB == nil || s.Catalog.DB == nil {
		return Claim{}, ErrClaimUnavailable
	}
	if !validID(in.OwnerUserID) || !validID(in.CouponID) || !validContext(in.CityCode, in.Business) || !validClaimKey(in.IdempotencyKey) {
		return Claim{}, ErrClaimInvalid
	}
	fingerprint := claimFingerprint(in)
	existing, err := s.Store.GetByKey(ctx, in.OwnerUserID, in.IdempotencyKey)
	if err == nil {
		if existing.RequestFingerprint != fingerprint || existing.CouponID != in.CouponID {
			return Claim{}, ErrClaimConflict
		}
		return s.resolve(ctx, existing)
	}
	if !errors.Is(err, ErrClaimNotFound) {
		return Claim{}, err
	}
	item, err := s.Catalog.Get(ctx, in.CouponID, in.CityCode, in.Business)
	if err != nil {
		return Claim{}, err
	}
	if item.ClaimMode != "IN_SITE_VERIFIED" {
		return Claim{}, ErrNotClaimable
	}
	id, err := newClaimID()
	if err != nil {
		return Claim{}, err
	}
	record, created, err := s.Store.Reserve(ctx, Claim{ID: id, OwnerUserID: in.OwnerUserID, CouponID: in.CouponID,
		IdempotencyKey: in.IdempotencyKey, RequestFingerprint: fingerprint})
	if err != nil {
		return Claim{}, err
	}
	if !created {
		return s.resolve(ctx, record)
	}
	item, err = s.Catalog.Get(ctx, in.CouponID, in.CityCode, in.Business)
	if errors.Is(err, ErrNotFound) || (err == nil && item.ClaimMode != "IN_SITE_VERIFIED") {
		return s.Store.SetOutcome(ctx, record.OwnerUserID, record.ID, "FAILED", "")
	}
	if err != nil {
		return Claim{}, err
	}
	outcome, err := s.Adapter.Claim(ctx, record)
	return s.apply(ctx, record, outcome, err)
}

func (s ClaimService) Get(ctx context.Context, owner, id string) (Claim, error) {
	record, err := s.Store.Get(ctx, owner, id)
	if err != nil {
		return Claim{}, err
	}
	return s.resolve(ctx, record)
}

func (s ClaimService) resolve(ctx context.Context, record Claim) (Claim, error) {
	if record.Status == "CLAIMED" || record.Status == "FAILED" || s.Adapter == nil {
		return record, nil
	}
	outcome, err := s.Adapter.Query(ctx, record)
	return s.apply(ctx, record, outcome, err)
}

func (s ClaimService) apply(ctx context.Context, record Claim, outcome Outcome, channelErr error) (Claim, error) {
	if channelErr != nil || (outcome.Status != "CLAIMED" && outcome.Status != "FAILED") ||
		(outcome.Status == "CLAIMED" && (!validText(outcome.EvidenceRef, 256) || strings.Contains(outcome.EvidenceRef, "://"))) {
		outcome = Outcome{Status: "QUERY_REQUIRED"}
	}
	if outcome.Status == "FAILED" {
		outcome.EvidenceRef = ""
	}
	updated, err := s.Store.SetOutcome(ctx, record.OwnerUserID, record.ID, outcome.Status, outcome.EvidenceRef)
	if errors.Is(err, ErrClaimConflict) {
		return s.Store.Get(ctx, record.OwnerUserID, record.ID)
	}
	return updated, err
}
