package conversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

var (
	ErrPriceChanged = errors.New("product price or rule changed")
	ErrUnavailable  = errors.New("conversion channel unavailable")
	ErrNotReady     = errors.New("conversion channel position not ready")
)

type ConvertInput struct {
	OwnerUserID    string
	PositionID     string
	PreviewID      string
	Scene          string
	IdempotencyKey string
}

type PreviewReader interface {
	FindByID(context.Context, string, string) (preview.Snapshot, error)
}

type RequestStore interface {
	Reserve(context.Context, ReserveInput) (Record, error)
	FindByKey(context.Context, string, string) (Record, error)
}

// Requoter uses the authorized channel API to refresh the exact product.
// It must not calculate a quote from client data or scrape a product page.
type Requoter interface {
	Requote(context.Context, string, preview.ChannelPosition) (preview.Quote, error)
}

type Service struct {
	Eligibility preview.Eligibility
	Previews    PreviewReader
	Requests    RequestStore
	Requoter    Requoter
}

func (s Service) Convert(ctx context.Context, in ConvertInput) (Record, error) {
	if !validText(in.OwnerUserID, 128) || !validText(in.PositionID, 128) ||
		!validText(in.PreviewID, 128) || !validText(in.Scene, 80) ||
		!validText(in.IdempotencyKey, 128) {
		return Record{}, ErrInvalid
	}
	if s.Eligibility == nil || s.Previews == nil || s.Requests == nil {
		return Record{}, ErrUnavailable
	}
	h := sha256.New()
	for _, part := range []string{in.PositionID, in.PreviewID, in.Scene} {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
	}
	fingerprint := hex.EncodeToString(h.Sum(nil))
	prior, err := s.Requests.FindByKey(ctx, in.OwnerUserID, in.IdempotencyKey)
	if err == nil {
		if prior.RequestFingerprint != fingerprint {
			return Record{}, ErrIdempotencyConflict
		}
		if _, err := s.Eligibility.Check(ctx, in.OwnerUserID, in.PositionID); err != nil {
			return Record{}, eligibilityError(err)
		}
		return prior, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	position, err := s.Eligibility.Check(ctx, in.OwnerUserID, in.PositionID)
	if err != nil {
		return Record{}, eligibilityError(err)
	}
	snapshot, err := s.Previews.FindByID(ctx, in.OwnerUserID, in.PreviewID)
	if errors.Is(err, preview.ErrNotFound) {
		return Record{}, ErrPreview
	}
	if err != nil {
		return Record{}, err
	}
	if snapshot.PositionID != in.PositionID || snapshot.Scene != in.Scene || snapshot.Channel != "JD" {
		return Record{}, ErrPreview
	}
	if !snapshot.ExpiresAt.After(time.Now()) {
		return Record{}, ErrExpired
	}
	if s.Requoter == nil {
		return Record{}, ErrUnavailable
	}
	quote, err := s.Requoter.Requote(ctx, snapshot.ExternalProductID, position)
	if err != nil {
		return Record{}, ErrUnavailable
	}
	if !quoteMatches(snapshot, quote) {
		return Record{}, ErrPriceChanged
	}
	if _, err := s.Eligibility.Check(ctx, in.OwnerUserID, in.PositionID); err != nil {
		return Record{}, eligibilityError(err)
	}
	return s.Requests.Reserve(ctx, ReserveInput{
		OwnerUserID: in.OwnerUserID, PositionID: in.PositionID,
		PreviewID: in.PreviewID, Scene: in.Scene,
		IdempotencyKey: in.IdempotencyKey, RequestFingerprint: fingerprint,
		ChannelAccountID: position.AccountID, ChannelPositionID: position.ExternalPositionID,
	})
}

func quoteMatches(snapshot preview.Snapshot, quote preview.Quote) bool {
	return quote.ExpiresAt.After(time.Now()) &&
		quote.ExternalProductID == snapshot.ExternalProductID &&
		quote.ProductName == snapshot.ProductName &&
		quote.CouponPriceMinor == snapshot.CouponPriceMinor &&
		quote.PromoterEstimateMinor == snapshot.PromoterEstimateMinor &&
		quote.ConsumerCashbackEstimateMinor == snapshot.ConsumerCashbackEstimateMinor &&
		quote.RuleVersion == snapshot.RuleVersion
}

func eligibilityError(err error) error {
	switch {
	case errors.Is(err, preview.ErrNotEnabled):
		return ErrNotEnabled
	case errors.Is(err, preview.ErrPosition):
		return ErrPosition
	case errors.Is(err, preview.ErrNotReady):
		return ErrNotReady
	default:
		return ErrUnavailable
	}
}
