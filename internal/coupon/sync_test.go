package coupon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type snapshotVerifier struct {
	snapshot VerifiedSnapshot
	err      error
}

func (v snapshotVerifier) Verify(context.Context, string) (VerifiedSnapshot, error) {
	return v.snapshot, v.err
}

func verifiedSample() VerifiedSnapshot {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return VerifiedSnapshot{
		Item: Item{ID: "JD:sku-1", Platform: "JD", ClaimMode: "BUNDLED_OFFER", Title: "已核验商品券",
			Scope: "PRODUCT", Currency: "CNY", DiscountMinor: 500, ThresholdMinor: 2000,
			RuleVersion: "rule-1", UpdatedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(time.Hour)},
		Products:    []Product{{ExternalProductID: "sku-1", Title: "商品一", UpdatedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(time.Hour)}},
		EvidenceRef: "jd-review-1", VerifiedAt: now.Add(-time.Minute), Active: true,
	}
}

func TestSyncRequiresVerifierAndRejectsInvalidProof(t *testing.T) {
	db := testDB(t)
	if err := (Synchronizer{DB: db}).Sync(context.Background(), "JD:sku-1"); err != ErrInvalid {
		t.Fatal(err)
	}
	s := verifiedSample()
	s.EvidenceRef = "https://secret.example/token"
	if err := (Synchronizer{DB: db, Verifier: snapshotVerifier{snapshot: s}}).Sync(context.Background(), s.Item.ID); err != ErrInvalid {
		t.Fatal(err)
	}
	s = verifiedSample()
	s.Item.Platform = "TB"
	if err := (Synchronizer{DB: db, Verifier: snapshotVerifier{snapshot: s}}).Sync(context.Background(), s.Item.ID); err != ErrInvalid {
		t.Fatal("platform/id mismatch accepted", err)
	}
	if err := (Synchronizer{DB: db, Verifier: snapshotVerifier{err: errors.New("upstream unavailable")}}).Sync(context.Background(), "JD:sku-1"); err == nil {
		t.Fatal("upstream error swallowed")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM coupon_catalog`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestVerifiedSyncReplayRevisionAndRevocation(t *testing.T) {
	db := testDB(t)
	s := verifiedSample()
	service := Synchronizer{DB: db, Verifier: snapshotVerifier{snapshot: s}}
	const workers = 12
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = service.Sync(context.Background(), s.Item.ID) }(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	catalog := Catalog{DB: db}
	page, err := catalog.List(context.Background(), ListInput{Platform: "JD", Limit: 20})
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM coupon_sync_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}

	newer := s
	newer.VerifiedAt = s.VerifiedAt.Add(30 * time.Second)
	newer.EvidenceRef = "jd-review-2"
	newer.Products = []Product{{ExternalProductID: "sku-2", Title: "商品二", UpdatedAt: s.Item.UpdatedAt, ExpiresAt: s.Item.ExpiresAt}}
	if err := (Synchronizer{DB: db, Verifier: snapshotVerifier{snapshot: newer}}).Sync(context.Background(), s.Item.ID); err != nil {
		t.Fatal(err)
	}
	products, err := catalog.Products(context.Background(), s.Item.ID, "", "", "", 20)
	if err != nil || len(products.Items) != 1 || products.Items[0].ExternalProductID != "sku-2" {
		t.Fatal(products, err)
	}
	if err := service.Sync(context.Background(), s.Item.ID); err != ErrStaleEvidence {
		t.Fatal("stale evidence accepted", err)
	}

	revoked := newer
	revoked.VerifiedAt = newer.VerifiedAt.Add(30 * time.Second)
	revoked.EvidenceRef = "jd-revoked-3"
	revoked.Active = false
	if err := (Synchronizer{DB: db, Verifier: snapshotVerifier{snapshot: revoked}}).Sync(context.Background(), s.Item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Get(context.Background(), s.Item.ID, "", ""); err != ErrNotFound {
		t.Fatal("revoked coupon visible", err)
	}
	if err := (Synchronizer{DB: db, Verifier: snapshotVerifier{snapshot: revoked}}).Sync(context.Background(), s.Item.ID); err != nil {
		t.Fatal("revoke replay failed", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM coupon_sync_events`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM coupon_products WHERE enabled`).Scan(&count); err != nil || count != 0 {
		t.Fatal("revoked products enabled", count, err)
	}
}
