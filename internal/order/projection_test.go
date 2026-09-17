package order

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestProjectionPreservesOrderAndRefundHistory(t *testing.T) {
	db := orderTestDB(t)
	store, err := NewStore(db, "test-v1", bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	projector := ProjectionStore{DB: db}
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	save := func(id, kind string, at time.Time) string {
		t.Helper()
		e, _, err := store.Save(context.Background(), RawEvent{Channel: "JD", EventID: id, ExternalOrderID: "order-1", EventType: kind, OccurredAt: at, Payload: []byte(fmt.Sprintf(`{"event":"%s"}`, id))})
		if err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	paid := save("paid", "ORDER", base.Add(2*time.Minute))
	result, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: paid, Status: "PAID"})
	if err != nil || result.Status != "PAID" || result.Disposition != "APPLIED" {
		t.Fatal(result, err)
	}
	paidAgain := save("paid-again", "ORDER", base.Add(150*time.Second))
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: paidAgain, Status: "PAID"})
	if err != nil || result.Status != "PAID" || result.Disposition != "DUPLICATE" {
		t.Fatal(result, err)
	}
	var observedAt time.Time
	if err := db.QueryRow(`SELECT status_at FROM normalized_orders WHERE channel='JD' AND external_order_id='order-1'`).Scan(&observedAt); err != nil || !observedAt.Equal(base.Add(150*time.Second)) {
		t.Fatal(observedAt, err)
	}
	older := save("created-late", "ORDER", base)
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: older, Status: "CREATED"})
	if err != nil || result.Status != "PAID" || result.Disposition != "STALE" {
		t.Fatal(result, err)
	}
	confirmed := save("confirmed", "ORDER", base.Add(3*time.Minute))
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: confirmed, Status: "CONFIRMED"})
	if err != nil || result.Status != "CONFIRMED" || result.Disposition != "APPLIED" {
		t.Fatal(result, err)
	}
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: confirmed, Status: "CONFIRMED"})
	if err != nil || result.Status != "CONFIRMED" || result.Disposition != "APPLIED" {
		t.Fatal("replay", result, err)
	}
	if _, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: confirmed, Status: "SETTLED"}); !errors.Is(err, ErrConflict) {
		t.Fatal("remap accepted", err)
	}
	lower := save("lower-new", "ORDER", base.Add(4*time.Minute))
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: lower, Status: "PAID"})
	if err != nil || result.Status != "CONFIRMED" || result.Disposition != "INVALID_TRANSITION" {
		t.Fatal(result, err)
	}
	partial := save("refund-partial", "REFUND", base.Add(5*time.Minute))
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: partial, RefundID: "refund-1", RefundKind: "PARTIAL", RefundAmountMinor: 100})
	if err != nil || result.Status != "CONFIRMED" {
		t.Fatal(result, err)
	}
	full := save("refund-full", "REFUND", base.Add(6*time.Minute))
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: full, RefundID: "refund-2", RefundKind: "FULL", RefundAmountMinor: 900})
	if err != nil || result.Status != "REFUNDED" {
		t.Fatal(result, err)
	}
	later := save("paid-after-refund", "ORDER", base.Add(7*time.Minute))
	result, err = projector.Apply(context.Background(), ProjectionInput{EvidenceID: later, Status: "PAID"})
	if err != nil || result.Status != "REFUNDED" || result.Disposition != "INVALID_TRANSITION" {
		t.Fatal(result, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM order_refund_events`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM order_projection_events`).Scan(&count); err != nil || count != 8 {
		t.Fatal(count, err)
	}
}

func TestProjectionRequiresSavedEvidenceAndSerializesConcurrentReplay(t *testing.T) {
	db := orderTestDB(t)
	store, err := NewStore(db, "test-v1", bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	projector := ProjectionStore{DB: db}
	if _, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: "missing", Status: "PAID"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	refund, _, err := store.Save(context.Background(), RawEvent{Channel: "TB", EventID: "refund-1", ExternalOrderID: "order-2", EventType: "REFUND", OccurredAt: at, Payload: []byte(`{"refund":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: refund.ID, RefundID: "r1", RefundKind: "PARTIAL", RefundAmountMinor: 1}); !errors.Is(err, ErrMissingOrder) {
		t.Fatal(err)
	}
	order, _, err := store.Save(context.Background(), RawEvent{Channel: "TB", EventID: "paid-1", ExternalOrderID: "order-2", EventType: "ORDER", OccurredAt: at.Add(-time.Minute), Payload: []byte(`{"paid":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 12
	var wg sync.WaitGroup
	results := make([]ProjectionResult, workers)
	errs := make([]error, workers)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = projector.Apply(context.Background(), ProjectionInput{EvidenceID: order.ID, Status: "PAID"})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil || results[i].Status != "PAID" {
			t.Fatal(i, results[i], err)
		}
	}
	if _, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: refund.ID, RefundID: "r1", RefundKind: "PARTIAL", RefundAmountMinor: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: refund.ID, RefundID: "r1", RefundKind: "FULL", RefundAmountMinor: 1}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := projector.Apply(context.Background(), ProjectionInput{EvidenceID: order.ID, RefundID: "wrong", RefundKind: "PARTIAL", RefundAmountMinor: 1}); !errors.Is(err, ErrProjectionInvalid) {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM order_projection_events`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
