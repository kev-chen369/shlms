package order

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/dashboard"
)

func TestAnomalyDuplicateReplayAndTimeBackfillKeepReadsConsistent(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	store, err := NewStore(db, "anomaly-v1", bytes.Repeat([]byte{77}, 32))
	if err != nil {
		t.Fatal(err)
	}
	owner, position, tracking, conversion := "anomaly-owner", "anomaly-position", "anomaly-tracking", "anomaly-conversion"
	seedAttributionFixture(t, db, owner, position, tracking, conversion)

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	dayStart := time.Date(2026, 9, 18, 0, 0, 0, 0, shanghai)
	previousDayEvent := dayStart.Add(-30 * time.Minute).UTC()
	paidAt := dayStart.Add(12 * time.Hour).UTC()
	save := func(event string, occurredAt time.Time, payload string) string {
		t.Helper()
		evidence, _, err := store.Save(ctx, RawEvent{Channel: "JD", EventID: event, ExternalOrderID: "anomaly-order", EventType: "ORDER", OccurredAt: occurredAt, Payload: []byte(payload)})
		if err != nil {
			t.Fatal(err)
		}
		return evidence.ID
	}
	paidID := save("anomaly-paid", paidAt, `{"status":"PAID"}`)
	if result, err := (ProjectionStore{DB: db}).Apply(ctx, ProjectionInput{EvidenceID: paidID, Status: "PAID"}); err != nil || result.Disposition != "APPLIED" {
		t.Fatal(result, err)
	}
	replayID := save("anomaly-paid-replay", paidAt.Add(time.Hour), `{"status":"PAID","replay":true}`)
	if result, err := (ProjectionStore{DB: db}).Apply(ctx, ProjectionInput{EvidenceID: replayID, Status: "PAID"}); err != nil || result.Disposition != "DUPLICATE" {
		t.Fatal(result, err)
	}
	earlyID := save("anomaly-created-late", previousDayEvent, `{"status":"CREATED"}`)
	if result, err := (ProjectionStore{DB: db}).Apply(ctx, ProjectionInput{EvidenceID: earlyID, Status: "CREATED"}); err != nil || result.Status != "PAID" || result.Disposition != "STALE" {
		t.Fatal(result, err)
	}
	if _, _, err := store.Save(ctx, RawEvent{Channel: "JD", EventID: "anomaly-paid", ExternalOrderID: "anomaly-order", EventType: "ORDER", OccurredAt: paidAt, Payload: []byte(`{"status":"PAID","tampered":true}`)}); !errors.Is(err, ErrConflict) {
		t.Fatalf("same event with changed payload accepted: %v", err)
	}

	if _, err := (AttributionStore{DB: db}).Apply(ctx, AttributionInput{EvidenceID: paidID, Method: AttributionSubID, Value: tracking}); err != nil {
		t.Fatal(err)
	}
	conversionAt := dayStart.Add(-time.Hour)
	if _, err := db.Exec(`UPDATE promotion_conversion_requests SET status='SUCCEEDED',channel_request_id='anomaly-channel-request',link_url='https://approved.example/anomaly',updated_at=$2 WHERE id=$1`, conversion, conversionAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO promotion_share_events(owner_user_id,event_id,conversion_id,action,scene,recorded_at) VALUES($1,'anomaly-copy',$2,'copy_link','anomaly',$3)`, owner, conversion, conversionAt); err != nil {
		t.Fatal(err)
	}

	reader := ReadStore{DB: db}
	previousPage, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: owner, From: dayStart.Add(-24 * time.Hour), To: dayStart, Limit: 20})
	if err != nil || len(previousPage.Items) != 1 || previousPage.Items[0].Status != "PAID" {
		t.Fatal(previousPage, err)
	}
	currentPage, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: owner, From: dayStart, To: dayStart.Add(24 * time.Hour), Limit: 20})
	if err != nil || len(currentPage.Items) != 0 {
		t.Fatal(currentPage, err)
	}
	detail, err := reader.GetOwned(ctx, owner, previousPage.Items[0].ID)
	expectedEarly := previousDayEvent.UTC().Truncate(time.Microsecond)
	if err != nil || detail.Status != "PAID" || !detail.OrderOccurredAt.Equal(expectedEarly) || len(detail.History) != 1 || detail.History[0].PreviousStatus != "" || detail.History[0].Status != "PAID" || !detail.History[0].OccurredAt.Equal(paidAt.UTC().Truncate(time.Microsecond)) {
		t.Fatal(detail, err)
	}
	counts, err := (dashboard.ReadStore{DB: db}).Get(ctx, dashboard.Filter{OwnerUserID: owner, From: dayStart.Add(-24 * time.Hour), To: dayStart})
	if err != nil || counts.SuccessfulLinks != 1 || counts.CopyReports != 1 || counts.ValidOrders != 1 {
		t.Fatal(counts, err)
	}
	currentCounts, err := (dashboard.ReadStore{DB: db}).Get(ctx, dashboard.Filter{OwnerUserID: owner, From: dayStart, To: dayStart.Add(24 * time.Hour)})
	if err != nil || currentCounts.SuccessfulLinks != 0 || currentCounts.CopyReports != 0 || currentCounts.ValidOrders != 0 {
		t.Fatal(currentCounts, err)
	}
	var projectionCount int
	if err := db.QueryRow(`SELECT count(*) FROM order_projection_events WHERE channel='JD' AND external_order_id='anomaly-order'`).Scan(&projectionCount); err != nil || projectionCount != 3 {
		t.Fatal(projectionCount, err)
	}
	for _, check := range []struct {
		evidence, status, disposition string
	}{
		{paidID, "PAID", "APPLIED"},
		{replayID, "PAID", "DUPLICATE"},
		{earlyID, "PAID", "STALE"},
	} {
		var resulting, disposition string
		if err := db.QueryRow(`SELECT resulting_status,disposition FROM order_projection_events WHERE evidence_id=$1`, check.evidence).Scan(&resulting, &disposition); err != nil || resulting != check.status || disposition != check.disposition {
			t.Fatalf("projection %s: got %s/%s want %s/%s: %v", check.evidence, resulting, disposition, check.status, check.disposition, err)
		}
	}
}

func TestAnomalyRefundsAndLateOrderDoNotReopenOrHalfWrite(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	store, err := NewStore(db, "refund-v1", bytes.Repeat([]byte{78}, 32))
	if err != nil {
		t.Fatal(err)
	}
	owner, position, tracking, conversion := "refund-owner", "refund-position", "refund-tracking", "refund-conversion"
	seedAttributionFixture(t, db, owner, position, tracking, conversion)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	dayStart := time.Date(2026, 9, 18, 0, 0, 0, 0, shanghai)
	orderAt := dayStart.Add(time.Hour).UTC()
	save := func(event, external, eventType string, at time.Time, payload string) string {
		t.Helper()
		evidence, _, err := store.Save(ctx, RawEvent{Channel: "JD", EventID: event, ExternalOrderID: external, EventType: eventType, OccurredAt: at, Payload: []byte(payload)})
		if err != nil {
			t.Fatal(err)
		}
		return evidence.ID
	}
	project := ProjectionStore{DB: db}
	paidID := save("refund-paid", "refund-order", "ORDER", orderAt, `{"status":"PAID"}`)
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: paidID, Status: "PAID"}); err != nil || result.Status != "PAID" {
		t.Fatal(result, err)
	}
	if _, err := (AttributionStore{DB: db}).Apply(ctx, AttributionInput{EvidenceID: paidID, Method: AttributionSubID, Value: tracking}); err != nil {
		t.Fatal(err)
	}
	partialID := save("refund-partial", "refund-order", "REFUND", orderAt.Add(time.Hour), `{"refund":"partial"}`)
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: partialID, RefundID: "refund-partial-id", RefundKind: "PARTIAL", RefundAmountMinor: 100}); err != nil || result.Status != "PAID" || result.Disposition != "APPLIED" {
		t.Fatal(result, err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM normalized_orders WHERE channel='JD' AND external_order_id='refund-order'`).Scan(&status); err != nil || status != "PAID" {
		t.Fatal(status, err)
	}
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: partialID, RefundID: "refund-partial-id", RefundKind: "PARTIAL", RefundAmountMinor: 100}); err != nil || result.Disposition != "APPLIED" {
		t.Fatal("partial refund replay", result, err)
	}
	var refundCount int
	if err := db.QueryRow(`SELECT count(*) FROM order_refund_events WHERE channel='JD' AND external_order_id='refund-order'`).Scan(&refundCount); err != nil || refundCount != 1 {
		t.Fatal(refundCount, err)
	}
	reader := ReadStore{DB: db}
	page, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: owner, From: dayStart, To: dayStart.Add(24 * time.Hour), Limit: 20})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != "PAID" {
		t.Fatal(page, err)
	}
	counts, err := (dashboard.ReadStore{DB: db}).Get(ctx, dashboard.Filter{OwnerUserID: owner, From: dayStart, To: dayStart.Add(24 * time.Hour)})
	if err != nil || counts.ValidOrders != 1 {
		t.Fatal(counts, err)
	}
	fullID := save("refund-full", "refund-order", "REFUND", orderAt.Add(2*time.Hour), `{"refund":"full"}`)
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: fullID, RefundID: "refund-full-id", RefundKind: "FULL", RefundAmountMinor: 900}); err != nil || result.Status != "REFUNDED" {
		t.Fatal(result, err)
	}
	latePaidID := save("refund-paid-late", "refund-order", "ORDER", orderAt.Add(3*time.Hour), `{"status":"PAID","late":true}`)
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: latePaidID, Status: "PAID"}); err != nil || result.Status != "REFUNDED" || result.Disposition != "INVALID_TRANSITION" {
		t.Fatal(result, err)
	}
	detail, err := reader.GetOwned(ctx, owner, page.Items[0].ID)
	if err != nil || detail.Status != "REFUNDED" || detail.RefundEventCount != 2 || len(detail.History) != 3 {
		t.Fatal(detail, err)
	}
	counts, err = (dashboard.ReadStore{DB: db}).Get(ctx, dashboard.Filter{OwnerUserID: owner, From: dayStart, To: dayStart.Add(24 * time.Hour)})
	if err != nil || counts.ValidOrders != 0 {
		t.Fatal(counts, err)
	}
	var effectiveRefunds int
	if err := db.QueryRow(`SELECT count(*) FROM order_refund_events WHERE channel='JD' AND external_order_id='refund-order'`).Scan(&effectiveRefunds); err != nil || effectiveRefunds != 2 {
		t.Fatal(effectiveRefunds, err)
	}

	lateRefundID := save("late-refund-first", "late-refund-order", "REFUND", orderAt, `{"refund":"before-order"}`)
	if _, err := project.Apply(ctx, ProjectionInput{EvidenceID: lateRefundID, RefundID: "late-refund-id", RefundKind: "PARTIAL", RefundAmountMinor: 50}); !errors.Is(err, ErrMissingOrder) {
		t.Fatalf("refund before order was accepted: %v", err)
	}
	var halfWritten int
	if err := db.QueryRow(`SELECT count(*) FROM order_projection_events WHERE evidence_id=$1`, lateRefundID).Scan(&halfWritten); err != nil || halfWritten != 0 {
		t.Fatal("refund-before-order left projection", halfWritten, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM order_refund_events WHERE evidence_id=$1`, lateRefundID).Scan(&halfWritten); err != nil || halfWritten != 0 {
		t.Fatal("refund-before-order left refund row", halfWritten, err)
	}
	lateOrderID := save("late-order-paid", "late-refund-order", "ORDER", orderAt.Add(time.Minute), `{"status":"PAID"}`)
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: lateOrderID, Status: "PAID"}); err != nil || result.Status != "PAID" {
		t.Fatal(result, err)
	}
	if result, err := project.Apply(ctx, ProjectionInput{EvidenceID: lateRefundID, RefundID: "late-refund-id", RefundKind: "PARTIAL", RefundAmountMinor: 50}); err != nil || result.Status != "PAID" {
		t.Fatal("late refund replay after order", result, err)
	}
}
