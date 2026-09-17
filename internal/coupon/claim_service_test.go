package coupon

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type claimAdapterStub struct {
	claimCalls, queryCalls     int
	claimOutcome, queryOutcome Outcome
	claimErr, queryErr         error
}

func (a *claimAdapterStub) Claim(context.Context, Claim) (Outcome, error) {
	a.claimCalls++
	return a.claimOutcome, a.claimErr
}
func (a *claimAdapterStub) Query(context.Context, Claim) (Outcome, error) {
	a.queryCalls++
	return a.queryOutcome, a.queryErr
}

func TestClaimServiceUnknownThenQueriesOriginalRequest(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO coupon_catalog(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
		VALUES('JD:claimable','JD','IN_SITE_VERIFIED','可核实券','ACTIVITY',100,1000,'v1','review-1',now(),now()-interval '1 hour',now()+interval '1 day',true),
		('JD:external','JD','PLATFORM_CLAIM','外部券','ACTIVITY',100,1000,'v1','review-2',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &claimAdapterStub{claimErr: errors.New("timeout"), queryOutcome: Outcome{Status: "CLAIMED", EvidenceRef: "official-receipt-1"}}
	service := ClaimService{Catalog: Catalog{DB: db}, Store: ClaimStore{DB: db}, Adapter: adapter}
	in := ClaimInput{OwnerUserID: "u1", CouponID: "JD:claimable", IdempotencyKey: "key-1"}
	first, err := service.Create(context.Background(), in)
	if err != nil || first.Status != "QUERY_REQUIRED" || adapter.claimCalls != 1 {
		t.Fatal(first, err, adapter.claimCalls)
	}
	replay, err := service.Create(context.Background(), in)
	if err != nil || replay.ID != first.ID || replay.Status != "CLAIMED" || adapter.claimCalls != 1 || adapter.queryCalls != 1 {
		t.Fatal(replay, err, adapter)
	}
	if _, err := service.Create(context.Background(), in); err != nil || adapter.queryCalls != 1 {
		t.Fatal("terminal request queried again", err)
	}
	changed := in
	changed.CityCode = "110100"
	if _, err := service.Create(context.Background(), changed); err != ErrClaimConflict {
		t.Fatal(err)
	}
	external := in
	external.CouponID = "JD:external"
	external.IdempotencyKey = "key-2"
	if _, err := service.Create(context.Background(), external); err != ErrNotClaimable {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), "u2", first.ID); err != ErrClaimNotFound {
		t.Fatal("cross-user claim visible", err)
	}
	noAdapter := ClaimService{Catalog: Catalog{DB: db}, Store: ClaimStore{DB: db}}
	if _, err := noAdapter.Create(context.Background(), ClaimInput{OwnerUserID: "u1", CouponID: "JD:claimable", IdempotencyKey: "key-3"}); err != ErrClaimUnavailable {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM coupon_claims`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestClaimServiceRequiresTrustedReceipt(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO coupon_catalog(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
		VALUES('JD:claimable','JD','IN_SITE_VERIFIED','可核实券','ACTIVITY',100,1000,'v1','review-1',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &claimAdapterStub{claimOutcome: Outcome{Status: "CLAIMED", EvidenceRef: "https://untrusted.example/receipt"}, queryErr: errors.New("unknown")}
	service := ClaimService{Catalog: Catalog{DB: db}, Store: ClaimStore{DB: db}, Adapter: adapter}
	claim, err := service.Create(context.Background(), ClaimInput{OwnerUserID: "u1", CouponID: "JD:claimable", IdempotencyKey: "key-1"})
	if err != nil || claim.Status != "QUERY_REQUIRED" || claim.EvidenceRef != "" {
		t.Fatal(claim, err)
	}
	if _, err := service.Get(context.Background(), "u1", claim.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := (ClaimStore{DB: db}).Get(context.Background(), "u1", claim.ID)
	if err != nil || stored.Status != "QUERY_REQUIRED" || strings.Contains(stored.EvidenceRef, "http") {
		t.Fatal(stored, err)
	}
}
