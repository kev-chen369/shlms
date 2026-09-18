package order

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReadOwnedOrdersFiltersAndMasks(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	store, err := NewStore(db, "test-v1", bytes.Repeat([]byte{41}, 32))
	if err != nil {
		t.Fatal(err)
	}
	seedAttributionFixture(t, db, "owner-read", "position-read", "tracking-read", "conversion-read")
	orderTime := time.Now().UTC().Add(-2 * time.Hour)
	create := func(event, external, status string, attributed bool) string {
		t.Helper()
		raw, _, err := store.Save(ctx, RawEvent{Channel: "JD", EventID: event, ExternalOrderID: external,
			EventType: "ORDER", OccurredAt: orderTime, Payload: []byte(`{"event":"` + event + `"}`)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := (ProjectionStore{DB: db}).Apply(ctx, ProjectionInput{EvidenceID: raw.ID, Status: status}); err != nil {
			t.Fatal(err)
		}
		method, value := AttributionNone, ""
		if attributed {
			method, value = AttributionSubID, "tracking-read"
		}
		if _, err := (AttributionStore{DB: db}).Apply(ctx, AttributionInput{EvidenceID: raw.ID, Method: method, Value: value}); err != nil {
			t.Fatal(err)
		}
		return raw.ID
	}
	create("read-event-1", "sensitive-order-1234", "PAID", true)
	create("read-event-2", "sensitive-order-5678", "CONFIRMED", true)
	create("read-event-3", "private-other-9999", "PAID", false)
	reader := ReadStore{DB: db}
	page, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: "owner-read", Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	if strings.Contains(page.Items[0].ID, "sensitive") || strings.Contains(page.Items[0].MaskedOrderID, "sensitive") {
		t.Fatal("raw order leaked", page.Items[0])
	}
	decoded, err := base64.RawURLEncoding.DecodeString(page.NextCursor)
	if err != nil || bytes.Contains(decoded, []byte("sensitive")) {
		t.Fatal("cursor leaked order id", err)
	}
	second, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: "owner-read", Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(second.Items) != 1 || second.Items[0].ID == page.Items[0].ID || second.NextCursor != "" {
		t.Fatal(second, err)
	}
	filtered, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: "owner-read", Limit: 20, Channel: "JD", PositionID: "position-read", Status: "CONFIRMED"})
	if err != nil || len(filtered.Items) != 1 || filtered.Items[0].Status != "CONFIRMED" {
		t.Fatal(filtered, err)
	}
	fromFuture, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: "owner-read", Limit: 20, From: time.Now().Add(time.Hour)})
	if err != nil || len(fromFuture.Items) != 0 {
		t.Fatal(fromFuture, err)
	}
	fromAfterOrder, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: "owner-read", Limit: 20, From: time.Now().Add(-time.Hour)})
	if err != nil || len(fromAfterOrder.Items) != 0 {
		t.Fatal("date filter used attribution time", fromAfterOrder, err)
	}
	detail, err := reader.GetOwned(ctx, "owner-read", filtered.Items[0].ID)
	if err != nil || detail.Status != "CONFIRMED" || len(detail.History) != 1 || detail.MaskedOrderID != "****5678" {
		t.Fatal(detail, err)
	}
	if _, err := reader.GetOwned(ctx, "another-user", filtered.Items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-user order visible", err)
	}
	if _, err := reader.GetOwned(ctx, "owner-read", "sensitive-order-5678"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := reader.ListOwned(ctx, OrderFilter{OwnerUserID: "owner-read", Limit: 20, Cursor: "invalid"}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
