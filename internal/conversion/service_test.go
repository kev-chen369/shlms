package conversion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

type eligibilityFunc func(context.Context, string, string) (preview.ChannelPosition, error)

func (f eligibilityFunc) Check(ctx context.Context, owner, position string) (preview.ChannelPosition, error) {
	return f(ctx, owner, position)
}

type previewFunc func(context.Context, string, string) (preview.Snapshot, error)

func (f previewFunc) FindByID(ctx context.Context, owner, id string) (preview.Snapshot, error) {
	return f(ctx, owner, id)
}

type requoterFunc func(context.Context, string, preview.ChannelPosition) (preview.Quote, error)

func (f requoterFunc) Requote(ctx context.Context, product string, p preview.ChannelPosition) (preview.Quote, error) {
	return f(ctx, product, p)
}

type requestStore struct {
	saved  *Record
	writes int
}

func (s *requestStore) FindByKey(_ context.Context, owner, key string) (Record, error) {
	if s.saved == nil || s.saved.OwnerUserID != owner || s.saved.IdempotencyKey != key {
		return Record{}, ErrNotFound
	}
	return *s.saved, nil
}
func (s *requestStore) Reserve(_ context.Context, in ReserveInput) (Record, error) {
	s.writes++
	r := Record{ID: "cr1", TrackingID: "tr1", OwnerUserID: in.OwnerUserID, PositionID: in.PositionID,
		PreviewID: in.PreviewID, Scene: in.Scene, IdempotencyKey: in.IdempotencyKey,
		RequestFingerprint: in.RequestFingerprint, Status: "PENDING"}
	s.saved = &r
	return r, nil
}

func TestConvertRequotesBeforeReserveAndReplays(t *testing.T) {
	ctx := context.Background()
	in := ConvertInput{OwnerUserID: "u1", PositionID: "p1", PreviewID: "pv1", Scene: "home", IdempotencyKey: "key"}
	now := time.Now().UTC()
	snapshot := preview.Snapshot{ID: "pv1", OwnerUserID: "u1", PositionID: "p1", Scene: "home", Channel: "JD",
		ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000,
		PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: now.Add(10 * time.Minute)}
	quote := preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000,
		PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: now.Add(10 * time.Minute)}
	store := &requestStore{}
	checks, requotes := 0, 0
	s := Service{
		Eligibility: eligibilityFunc(func(_ context.Context, owner, position string) (preview.ChannelPosition, error) {
			checks++
			if owner != "u1" || position != "p1" {
				t.Fatal(owner, position)
			}
			return preview.ChannelPosition{ExternalPositionID: "ext"}, nil
		}),
		Previews: previewFunc(func(_ context.Context, owner, id string) (preview.Snapshot, error) {
			if owner != "u1" || id != "pv1" {
				t.Fatal(owner, id)
			}
			return snapshot, nil
		}),
		Requests: store,
		Requoter: requoterFunc(func(_ context.Context, product string, p preview.ChannelPosition) (preview.Quote, error) {
			requotes++
			if product != "sku" || p.ExternalPositionID != "ext" {
				t.Fatal(product, p)
			}
			return quote, nil
		}),
	}
	first, err := s.Convert(ctx, in)
	if err != nil || first.ID != "cr1" || store.writes != 1 || checks != 2 || requotes != 1 {
		t.Fatal(first, err, store.writes, checks, requotes)
	}
	second, err := s.Convert(ctx, in)
	if err != nil || second.ID != first.ID || store.writes != 1 || requotes != 1 {
		t.Fatal(second, err, store.writes, requotes)
	}
	in.Scene = "other"
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	in.Scene = "home"
	in.PositionID = "p2"
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal("changed position did not return idempotency conflict", err)
	}
	if store.writes != 1 || requotes != 1 {
		t.Fatal("conflicting replay performed a second conversion", store.writes, requotes)
	}
}

func TestConvertFailsClosedBeforeTracking(t *testing.T) {
	ctx := context.Background()
	in := ConvertInput{OwnerUserID: "u1", PositionID: "p1", PreviewID: "pv1", Scene: "home", IdempotencyKey: "key"}
	now := time.Now().UTC()
	snapshot := preview.Snapshot{ID: "pv1", OwnerUserID: "u1", PositionID: "p1", Scene: "home", Channel: "JD",
		ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000, RuleVersion: "v1", ExpiresAt: now.Add(time.Minute)}
	quote := preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000, RuleVersion: "v1", ExpiresAt: now.Add(time.Minute)}
	store := &requestStore{}
	s := Service{Eligibility: eligibilityFunc(func(context.Context, string, string) (preview.ChannelPosition, error) {
		return preview.ChannelPosition{}, nil
	}),
		Previews: previewFunc(func(context.Context, string, string) (preview.Snapshot, error) { return snapshot, nil }), Requests: store}
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrUnavailable) || store.writes != 0 {
		t.Fatal(err, store.writes)
	}
	s.Requoter = requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) { return quote, nil })
	quote.CouponPriceMinor = 1100
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrPriceChanged) || store.writes != 0 {
		t.Fatal(err, store.writes)
	}
	quote.CouponPriceMinor = 1000
	snapshot.ExpiresAt = now.Add(-time.Minute)
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrExpired) || store.writes != 0 {
		t.Fatal(err, store.writes)
	}
	snapshot.ExpiresAt = now.Add(time.Minute)
	s.Eligibility = eligibilityFunc(func(context.Context, string, string) (preview.ChannelPosition, error) {
		return preview.ChannelPosition{}, preview.ErrNotReady
	})
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrNotReady) || store.writes != 0 {
		t.Fatal(err, store.writes)
	}
	in.PreviewID = ""
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestConvertRechecksEligibilityAfterRequote(t *testing.T) {
	now := time.Now().UTC()
	checks := 0
	store := &requestStore{}
	s := Service{
		Eligibility: eligibilityFunc(func(context.Context, string, string) (preview.ChannelPosition, error) {
			checks++
			if checks == 2 {
				return preview.ChannelPosition{}, preview.ErrNotReady
			}
			return preview.ChannelPosition{ExternalPositionID: "ext"}, nil
		}),
		Previews: previewFunc(func(context.Context, string, string) (preview.Snapshot, error) {
			return preview.Snapshot{PositionID: "p1", Scene: "home", Channel: "JD", ExternalProductID: "sku",
				ProductName: "product", CouponPriceMinor: 1000, RuleVersion: "v1", ExpiresAt: now.Add(time.Minute)}, nil
		}),
		Requoter: requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) {
			return preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000,
				RuleVersion: "v1", ExpiresAt: now.Add(time.Minute)}, nil
		}),
		Requests: store,
	}
	_, err := s.Convert(context.Background(), ConvertInput{OwnerUserID: "u1", PositionID: "p1", PreviewID: "pv1", Scene: "home", IdempotencyKey: "key"})
	if !errors.Is(err, ErrNotReady) || checks != 2 || store.writes != 0 {
		t.Fatal(err, checks, store.writes)
	}
}

func TestConvertWithPostgresPersistsOnlyAfterMatchingQuote(t *testing.T) {
	db := conversionDB(t)
	ctx := context.Background()
	previews := preview.NewRepository(db)
	s := Service{
		Eligibility: eligibilityFunc(func(context.Context, string, string) (preview.ChannelPosition, error) {
			return preview.ChannelPosition{AccountID: "account", ExternalPositionID: "position"}, nil
		}),
		Previews: previews, Requests: Repository{DB: db},
	}
	in := ConvertInput{OwnerUserID: "u1", PositionID: "pos-u1", PreviewID: "pv-u1", Scene: "home", IdempotencyKey: "service-key"}
	s.Requoter = requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) {
		return preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1001,
			PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: time.Now().Add(time.Minute)}, nil
	})
	if _, err := s.Convert(ctx, in); !errors.Is(err, ErrPriceChanged) {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	s.Requoter = requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) {
		return preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000,
			PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: time.Now().Add(time.Minute)}, nil
	})
	result, err := s.Convert(ctx, in)
	if err != nil || result.Status != "PENDING" {
		t.Fatal(result, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	again, err := s.Convert(ctx, in)
	if err != nil || again.ID != result.ID {
		t.Fatal(again, err)
	}
	conflict := in
	conflict.PositionID = "pos-u2"
	if _, err := s.Convert(ctx, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal("reused key with different position", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 1 {
		t.Fatal("conflicting key wrote tracking", count, err)
	}
}
