package coupon

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestClaimReserveIsolationAndOutcomes(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO coupon_catalog
		(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
		VALUES('JD:claim-1','JD','IN_SITE_VERIFIED','测试券','ACTIVITY',100,1000,'v1','review-1',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
	if err != nil {
		t.Fatal(err)
	}
	store := ClaimStore{DB: db}
	base := Claim{ID: "claim-1", OwnerUserID: "u1", CouponID: "JD:claim-1", IdempotencyKey: "claim-key-1", RequestFingerprint: strings.Repeat("a", 64)}
	const workers = 12
	var wg sync.WaitGroup
	errs := make([]error, workers)
	created := make([]bool, workers)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, created[i], errs[i] = store.Reserve(context.Background(), base) }(i)
	}
	wg.Wait()
	countCreated := 0
	for i, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
		if created[i] {
			countCreated++
		}
	}
	if countCreated != 1 {
		t.Fatal("expected one upstream invocation", countCreated)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM coupon_claims`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	conflict := base
	conflict.RequestFingerprint = strings.Repeat("b", 64)
	if _, _, err := store.Reserve(context.Background(), conflict); err != ErrClaimConflict {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "u2", base.ID); err != ErrClaimNotFound {
		t.Fatal("cross-user claim visible", err)
	}
	if _, err := store.SetOutcome(context.Background(), "u1", base.ID, "CLAIMED", ""); err != ErrClaimInvalid {
		t.Fatal(err)
	}
	unknown, err := store.SetOutcome(context.Background(), "u1", base.ID, "QUERY_REQUIRED", "")
	if err != nil || unknown.Status != "QUERY_REQUIRED" {
		t.Fatal(unknown, err)
	}
	claimed, err := store.SetOutcome(context.Background(), "u1", base.ID, "CLAIMED", "official-receipt-1")
	if err != nil || claimed.Status != "CLAIMED" || claimed.EvidenceRef != "official-receipt-1" {
		t.Fatal(claimed, err)
	}
	if _, err := store.SetOutcome(context.Background(), "u1", base.ID, "FAILED", ""); err != ErrClaimConflict {
		t.Fatal("terminal status changed", err)
	}
	if _, err := store.SetOutcome(context.Background(), "u1", base.ID, "CLAIMED", "official-receipt-1"); err != nil {
		t.Fatal("terminal replay failed", err)
	}
}
