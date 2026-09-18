package conversion

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestShareEventOwnedSuccessfulAndConcurrentDedup(t *testing.T) {
	db := conversionDB(t)
	_, err := db.Exec(`INSERT INTO tracking_records(id,idempotency_key,user_id,channel,external_product_id,source,created_at) VALUES('event-tracking','event-key','u1','JD','sku','PROMOTION_CENTER',CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO promotion_conversion_requests(id,owner_user_id,position_id,preview_id,tracking_id,idempotency_key,request_fingerprint,scene,status,channel_request_id,link_url,created_at,updated_at) VALUES('event-request','u1','pos-u1','pv-u1','event-tracking','event-convert','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','home','SUCCEEDED','event-request','https://approved.example/item',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	store := ShareEventStore{DB: db}
	in := ShareEventInput{OwnerUserID: "u1", RequestID: "event-request", EventID: "event-1", ArtifactType: "link", Scene: "home", Action: "COPY_REPORTED"}
	var wg sync.WaitGroup
	results := make(chan ShareEvent, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); v, e := store.Record(context.Background(), in); results <- v; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var first ShareEvent
	for v := range results {
		if first.RecordedAt.IsZero() {
			first = v
		}
		if v.RecordedAt != first.RecordedAt || v.Action != "COPY_REPORTED" {
			t.Fatal(v, first)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM promotion_share_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	in.ArtifactType = "text"
	if _, err := store.Record(context.Background(), in); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	in.EventID = "event-2"
	in.OwnerUserID = "u2"
	if _, err := store.Record(context.Background(), in); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	in.OwnerUserID = "u1"
	if _, err := db.Exec(`UPDATE promotion_conversion_requests SET status='FAILED_FINAL',link_url=NULL WHERE id='event-request'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Record(context.Background(), in); !errors.Is(err, ErrShareNotReady) {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM promotion_share_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestShareEventContractsShareIdempotency(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	link := succeededShareRequest(t, repo, request())
	ctx := context.Background()
	native := NativeShareEventInput{OwnerUserID: "u1", LinkID: link.ID, EventID: "shared-event", Action: "copy_link", Scene: "home"}
	first, err := repo.RecordShareEvent(ctx, native)
	if err != nil {
		t.Fatal(err)
	}
	legacy := ShareEventInput{OwnerUserID: "u1", RequestID: link.ID, EventID: native.EventID, ArtifactType: "link", Scene: "home", Action: "COPY_REPORTED"}
	second, err := (ShareEventStore{DB: db}).Record(ctx, legacy)
	if err != nil || !second.RecordedAt.Equal(first.RecordedAt) {
		t.Fatal(second, err)
	}
	legacy.ArtifactType = "text"
	if _, err := (ShareEventStore{DB: db}).Record(ctx, legacy); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM promotion_share_events WHERE owner_user_id='u1' AND event_id='shared-event'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
