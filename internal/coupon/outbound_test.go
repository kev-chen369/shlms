package coupon

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type outboundResolverFunc func(context.Context, Item, OutboundInput) (OutboundTarget, error)

func (f outboundResolverFunc) ResolveOutbound(ctx context.Context, item Item, in OutboundInput) (OutboundTarget, error) {
	return f(ctx, item, in)
}

func TestOutboundPrepareIsBoundedAndIdempotent(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO coupon_catalog
		(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,city_code,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
		VALUES('MT:out-1','MT','PLATFORM_ACTIVITY','测试活动','ACTIVITY',100,1000,'110100','v1','e1',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
	if err != nil {
		t.Fatal(err)
	}
	resolver := outboundResolverFunc(func(_ context.Context, _ Item, _ OutboundInput) (OutboundTarget, error) {
		return OutboundTarget{URL: "https://approved.example/activity?a=1", EvidenceRef: "approved-response-1"}, nil
	})
	service := OutboundService{Catalog: Catalog{DB: db}, DB: db, Resolver: resolver,
		AllowedHosts: map[string]map[string]bool{"MT": {"approved.example": true}}}
	in := OutboundInput{OwnerKey: "session-1", CouponID: "MT:out-1", CityCode: "110100", Terminal: "H5", EntryPoint: "coupon-detail", IdempotencyKey: "out-1"}
	const workers = 8
	results := make([]OutboundResult, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = service.Prepare(context.Background(), in) }(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil || results[i].ID != results[0].ID || results[i].TargetHost != "approved.example" {
			t.Fatal(i, results[i], err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM coupon_outbounds`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	changed := in
	changed.Terminal = "APP"
	if _, err := service.Prepare(context.Background(), changed); !errors.Is(err, ErrOutboundConflict) {
		t.Fatal("changed request", err)
	}
	wrongCity := in
	wrongCity.CityCode = "310100"
	wrongCity.IdempotencyKey = "out-2"
	if _, err := service.Prepare(context.Background(), wrongCity); !errors.Is(err, ErrNotFound) {
		t.Fatal("city", err)
	}
	service.AllowedHosts["MT"] = map[string]bool{"other.example": true}
	in.IdempotencyKey = "out-3"
	if _, err := service.Prepare(context.Background(), in); !errors.Is(err, ErrOutboundUnavailable) {
		t.Fatal("host", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM coupon_outbounds`).Scan(&count); err != nil || count != 1 {
		t.Fatal("unapproved target persisted", count, err)
	}
	service.Resolver = nil
	if _, err := service.Prepare(context.Background(), in); !errors.Is(err, ErrOutboundInvalid) {
		t.Fatal("unconfigured resolver accepted", err)
	}
	_, err = db.Exec(`UPDATE coupon_catalog SET enabled=false WHERE id='MT:out-1'`)
	if err != nil {
		t.Fatal(err)
	}
	service.Resolver = resolver
	if _, err := service.Prepare(context.Background(), in); !errors.Is(err, ErrNotFound) {
		t.Fatal("withdrawn coupon accepted", err)
	}
}

func TestOutboundRejectsUnsafeTargets(t *testing.T) {
	for _, raw := range []string{"http://approved.example/a", "https://approved.example.evil.test/a", "https://user@approved.example/a", "https://approved.example:8443/a", "https://approved.example/a#fragment"} {
		t.Run(raw, func(t *testing.T) {
			db := testDB(t)
			_, err := db.Exec(`INSERT INTO coupon_catalog(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
				VALUES('JD:o','JD','BUNDLED_OFFER','券','ACTIVITY',0,0,'v1','e1',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
			if err != nil {
				t.Fatal(err)
			}
			s := OutboundService{Catalog: Catalog{DB: db}, DB: db,
				Resolver: outboundResolverFunc(func(context.Context, Item, OutboundInput) (OutboundTarget, error) {
					return OutboundTarget{URL: raw, EvidenceRef: "e1"}, nil
				}),
				AllowedHosts: map[string]map[string]bool{"JD": {"approved.example": true}}}
			_, err = s.Prepare(context.Background(), OutboundInput{OwnerKey: "u1", CouponID: "JD:o", Terminal: "H5", EntryPoint: "detail", IdempotencyKey: "key"})
			if !errors.Is(err, ErrOutboundUnavailable) {
				t.Fatal(err)
			}
		})
	}
}
