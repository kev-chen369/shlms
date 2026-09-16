package preview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/kev-chen369/shlms/internal/linkresolve"
)

var (
	ErrNotEnabled  = errors.New("promoter is not enabled")
	ErrPosition    = errors.New("position is unavailable")
	ErrNotReady    = errors.New("channel position is not ready")
	ErrUnavailable = errors.New("preview quote is unavailable")
)

type Request struct {
	OwnerUserID    string
	PositionID     string
	IdempotencyKey string
	Input          string
	Scene          string
}

type Eligibility interface {
	Check(context.Context, string, string) (ChannelPosition, error)
}

type ChannelPosition struct {
	AccountID          string
	ExternalPositionID string
}

// Resolver must enforce an approved-host policy, public DNS and bounded HTTPS redirects.
type Resolver interface {
	Resolve(context.Context, string) (string, error)
}

// Quoter obtains product data from an authorized channel API, not from client values.
type Quoter interface {
	Quote(context.Context, string, ChannelPosition) (Quote, error)
}

type Quote struct {
	ExternalProductID             string
	ProductName                   string
	CouponPriceMinor              int64
	PromoterEstimateMinor         int64
	ConsumerCashbackEstimateMinor int64
	RuleVersion                   string
	EvidenceRef                   string
	UpdatedAt                     time.Time
	ExpiresAt                     time.Time
}

type Store interface {
	Save(context.Context, Snapshot) (Snapshot, error)
	FindByKey(context.Context, string, string) (Snapshot, error)
}

type Service struct {
	Eligibility Eligibility
	Resolver    Resolver
	Quoter      Quoter
	Store       Store
}

func (s Service) Create(ctx context.Context, r Request) (Snapshot, error) {
	if !bounded(r.OwnerUserID, 128) || !bounded(r.PositionID, 128) ||
		!bounded(r.IdempotencyKey, 128) || !bounded(r.Scene, 80) ||
		len(r.Input) == 0 || len(r.Input) > 4096 || strings.TrimSpace(r.Input) != r.Input {
		return Snapshot{}, ErrInvalid
	}
	if s.Eligibility == nil || s.Store == nil {
		return Snapshot{}, ErrUnavailable
	}
	position, err := s.Eligibility.Check(ctx, r.OwnerUserID, r.PositionID)
	if err != nil {
		return Snapshot{}, err
	}
	// The key binds the exact input, position and scene. No price or owner comes from the client.
	h := sha256.New()
	for _, part := range []string{r.PositionID, r.Scene, r.Input} {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
	}
	fingerprint := hex.EncodeToString(h.Sum(nil))
	prior, err := s.Store.FindByKey(ctx, r.OwnerUserID, r.IdempotencyKey)
	if err == nil {
		if prior.RequestFingerprint != fingerprint {
			return Snapshot{}, ErrIdempotencyConflict
		}
		return prior, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Snapshot{}, err
	}
	if s.Resolver == nil || s.Quoter == nil {
		return Snapshot{}, ErrUnavailable
	}
	finalURL, err := s.Resolver.Resolve(ctx, r.Input)
	if err != nil {
		if errors.Is(err, linkresolve.ErrURLRejected) || errors.Is(err, linkresolve.ErrAddressRejected) {
			return Snapshot{}, ErrInvalid
		}
		return Snapshot{}, ErrUnavailable
	}
	quote, err := s.Quoter.Quote(ctx, finalURL, position)
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return Snapshot{}, err
	}
	result := Snapshot{
		ID: "PV-" + hex.EncodeToString(id[:]), OwnerUserID: r.OwnerUserID,
		PositionID: r.PositionID, IdempotencyKey: r.IdempotencyKey,
		RequestFingerprint: fingerprint, Scene: r.Scene, Channel: "JD",
		ExternalProductID: quote.ExternalProductID, ProductName: quote.ProductName,
		Currency: "CNY", CouponPriceMinor: quote.CouponPriceMinor,
		PromoterEstimateMinor:         quote.PromoterEstimateMinor,
		ConsumerCashbackEstimateMinor: quote.ConsumerCashbackEstimateMinor,
		RuleVersion:                   quote.RuleVersion, EvidenceRef: quote.EvidenceRef,
		UpdatedAt: quote.UpdatedAt, ExpiresAt: quote.ExpiresAt,
	}
	if !result.valid() || !result.ExpiresAt.After(time.Now()) {
		return Snapshot{}, ErrUnavailable
	}
	// Recheck after the upstream call so a disabled position cannot create a new quote.
	if _, err := s.Eligibility.Check(ctx, r.OwnerUserID, r.PositionID); err != nil {
		return Snapshot{}, err
	}
	return s.Store.Save(ctx, result)
}

func bounded(value string, max int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= max
}
