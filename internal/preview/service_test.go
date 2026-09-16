package preview

import (
	"context"
	"errors"
	"testing"
	"time"
)

type eligibilityFunc func(context.Context, string, string) (ChannelPosition, error)

func (f eligibilityFunc) Check(ctx context.Context, owner, position string) (ChannelPosition, error) {
	return f(ctx, owner, position)
}

type resolverFunc func(context.Context, string) (string, error)

func (f resolverFunc) Resolve(ctx context.Context, input string) (string, error) {
	return f(ctx, input)
}

type quoterFunc func(context.Context, string, ChannelPosition) (Quote, error)

func (f quoterFunc) Quote(ctx context.Context, link string, p ChannelPosition) (Quote, error) {
	return f(ctx, link, p)
}

type memoryStore struct {
	records map[string]Snapshot
	writes  int
}

func (m *memoryStore) FindByKey(_ context.Context, owner, key string) (Snapshot, error) {
	s, ok := m.records[owner+"/"+key]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	return s, nil
}
func (m *memoryStore) Save(_ context.Context, s Snapshot) (Snapshot, error) {
	m.writes++
	if existing, ok := m.records[s.OwnerUserID+"/"+s.IdempotencyKey]; ok {
		if existing.RequestFingerprint != s.RequestFingerprint {
			return Snapshot{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	m.records[s.OwnerUserID+"/"+s.IdempotencyKey] = s
	return s, nil
}

func TestServiceQuoteReplayAndFailClosed(t *testing.T) {
	ctx := context.Background()
	request := Request{OwnerUserID: "u1", PositionID: "p1", IdempotencyKey: "key", Input: "https://approved.example/item", Scene: "home"}
	store := &memoryStore{records: map[string]Snapshot{}}
	checks, resolves, quotes := 0, 0, 0
	s := Service{
		Eligibility: eligibilityFunc(func(_ context.Context, owner, position string) (ChannelPosition, error) {
			checks++
			if owner != "u1" || position != "p1" {
				t.Fatal(owner, position)
			}
			return ChannelPosition{AccountID: "account", ExternalPositionID: "ext-pos"}, nil
		}),
		Resolver: resolverFunc(func(_ context.Context, input string) (string, error) {
			resolves++
			if input != request.Input {
				t.Fatal(input)
			}
			return input, nil
		}),
		Quoter: quoterFunc(func(_ context.Context, input string, p ChannelPosition) (Quote, error) {
			quotes++
			if input != request.Input || p.ExternalPositionID != "ext-pos" {
				t.Fatal(input, p)
			}
			now := time.Now().UTC()
			return Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1200,
				PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1",
				EvidenceRef: "evidence", UpdatedAt: now, ExpiresAt: now.Add(10 * time.Minute)}, nil
		}),
		Store: store,
	}
	first, err := s.Create(ctx, request)
	if err != nil || first.ID == "" || first.OwnerUserID != "u1" || store.writes != 1 || checks != 2 {
		t.Fatal(first, err, store.writes, checks)
	}
	second, err := s.Create(ctx, request)
	if err != nil || second.ID != first.ID || store.writes != 1 || resolves != 1 || quotes != 1 {
		t.Fatal(second, err, store.writes, resolves, quotes)
	}
	request.PositionID = "p2"
	s.Eligibility = eligibilityFunc(func(context.Context, string, string) (ChannelPosition, error) { return ChannelPosition{}, nil })
	if _, err := s.Create(ctx, request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	request.PositionID = "p1"
	request.IdempotencyKey = "new-key"
	s.Quoter = nil
	if _, err := s.Create(ctx, request); !errors.Is(err, ErrUnavailable) || store.writes != 1 {
		t.Fatal(err, store.writes)
	}
	s.Eligibility = eligibilityFunc(func(context.Context, string, string) (ChannelPosition, error) { return ChannelPosition{}, ErrNotReady })
	if _, err := s.Create(ctx, request); !errors.Is(err, ErrNotReady) || store.writes != 1 {
		t.Fatal(err, store.writes)
	}
	request.Input = " "
	if _, err := s.Create(ctx, request); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestPostgresEligibilityFailClosed(t *testing.T) {
	db := previewDB(t)
	e := PostgresEligibility{DB: db}
	ctx := context.Background()
	if _, err := e.Check(ctx, "u2", "p1"); !errors.Is(err, ErrPosition) {
		t.Fatal(err)
	}
	if _, err := e.Check(ctx, "u1", "p1"); !errors.Is(err, ErrNotEnabled) {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status)
		VALUES('app1','u1','apply','User','home','v1',CURRENT_TIMESTAMP,'ENABLED')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE promoter_profiles SET status='ENABLED',application_id='app1',version=1 WHERE user_id='u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Check(ctx, "u1", "p1"); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO channel_positions(position_id,channel,account_id,external_position_id,version)
		VALUES('p1','JD','account','ext',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Check(ctx, "u1", "p1"); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}
