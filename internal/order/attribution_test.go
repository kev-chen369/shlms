package order

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestAttributionInputValidation(t *testing.T) {
	valid := []AttributionInput{
		{EvidenceID: "e1", Method: AttributionNone},
		{EvidenceID: "e1", Method: AttributionSubID, Value: "tracking-1"},
		{EvidenceID: "e1", Method: AttributionLinkRequest, Value: "request-1"},
		{EvidenceID: "e1", Method: AttributionChannelPosition, Value: "position-1", AccountID: "account-1"},
	}
	for _, in := range valid {
		if !in.valid() {
			t.Fatalf("valid input rejected: %#v", in)
		}
	}
	invalid := []AttributionInput{
		{},
		{EvidenceID: "e1", Method: "TIME_MATCH", Value: "x"},
		{EvidenceID: "e1", Method: AttributionNone, Value: "guess"},
		{EvidenceID: "e1", Method: AttributionSubID},
		{EvidenceID: "e1", Method: AttributionChannelPosition, Value: "position-1"},
	}
	for _, in := range invalid {
		if in.valid() {
			t.Fatalf("invalid input accepted: %#v", in)
		}
	}
}

func TestAttributionExactTrackingPendingAndConflict(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	store, err := NewStore(db, "test-v1", bytes.Repeat([]byte{31}, 32))
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := store.Save(ctx, RawEvent{Channel: "JD", EventID: "order-attribution-1", ExternalOrderID: "order-attribution-1",
		EventType: "ORDER", OccurredAt: time.Now().UTC(), Payload: []byte(`{"order":"order-attribution-1"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (ProjectionStore{DB: db}).Apply(ctx, ProjectionInput{EvidenceID: raw.ID, Status: "PAID"}); err != nil {
		t.Fatal(err)
	}
	seedAttributionFixture(t, db, "owner-a", "position-a", "tracking-a", "conversion-a")

	attributions := AttributionStore{DB: db}
	got, err := attributions.Apply(ctx, AttributionInput{EvidenceID: raw.ID, Method: AttributionSubID, Value: "tracking-a"})
	if err != nil || got.Status != "ATTRIBUTED" || got.OwnerUserID != "owner-a" || got.PositionID != "position-a" || got.TrackingID != "tracking-a" {
		t.Fatalf("attribution=%#v err=%v", got, err)
	}
	again, err := attributions.Apply(ctx, AttributionInput{EvidenceID: raw.ID, Method: AttributionSubID, Value: "tracking-a"})
	if err != nil || again != got {
		t.Fatalf("replay=%#v err=%v", again, err)
	}
	if _, err := attributions.Apply(ctx, AttributionInput{EvidenceID: raw.ID, Method: AttributionSubID, Value: "other"}); !errors.Is(err, ErrAttributionConflict) {
		t.Fatalf("conflict error=%v", err)
	}
}

func TestAttributionUnknownEvidenceWaitsForReview(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	store, err := NewStore(db, "test-v1", bytes.Repeat([]byte{32}, 32))
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := store.Save(ctx, RawEvent{Channel: "JD", EventID: "order-attribution-2", ExternalOrderID: "order-attribution-2",
		EventType: "ORDER", OccurredAt: time.Now().UTC(), Payload: []byte(`{"order":"order-attribution-2"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (ProjectionStore{DB: db}).Apply(ctx, ProjectionInput{EvidenceID: raw.ID, Status: "PAID"}); err != nil {
		t.Fatal(err)
	}
	got, err := (AttributionStore{DB: db}).Apply(ctx, AttributionInput{EvidenceID: raw.ID, Method: AttributionSubID, Value: "unknown"})
	if err != nil || got.Status != "PENDING_REVIEW" || got.OwnerUserID != "" || got.TrackingID != "" {
		t.Fatalf("attribution=%#v err=%v", got, err)
	}
}

func seedAttributionFixture(t *testing.T, db interface {
	Exec(string, ...any) (sql.Result, error)
}, owner, position, trackingID, conversionID string) {
	t.Helper()
	// Fixture rows deliberately use only database-owned identities. No buyer is inferred.
	queries := []struct {
		q string
		a []any
	}{
		{`INSERT INTO promoter_profiles(user_id,status,application_id,version) VALUES($1,'NOT_APPLIED',NULL,0)`, []any{owner}},
		{`INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status) VALUES('application-attribution',$1,'application-key','attribution','share','v1',CURRENT_TIMESTAMP,'ENABLED')`, []any{owner}},
		{`UPDATE promoter_profiles SET status='ENABLED',application_id='application-attribution',version=1 WHERE user_id=$1`, []any{owner}},
		{`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,is_default,version) VALUES($1,$2,'attribution','share','ENABLED',false,1)`, []any{position, owner}},
		{`INSERT INTO tracking_records(id,idempotency_key,user_id,channel,external_product_id,source,created_at) VALUES($1,$2,$3,'JD','sku','PROMOTION_CENTER',CURRENT_TIMESTAMP)`, []any{trackingID, "key-" + trackingID, owner}},
		{`INSERT INTO promotion_previews(id,owner_user_id,position_id,idempotency_key,request_fingerprint,scene,channel,external_product_id,product_name,currency,coupon_price_minor,promoter_estimate_minor,consumer_cashback_estimate_minor,rule_version,evidence_ref,updated_at,expires_at) VALUES('preview-attribution',$1,$2,'preview-key','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','share','JD','sku','title','CNY',100,1,0,'r1','e1',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP + interval '1 hour')`, []any{owner, position}},
		{`INSERT INTO promotion_conversion_requests(id,owner_user_id,position_id,preview_id,tracking_id,idempotency_key,request_fingerprint,scene,status,created_at,updated_at) VALUES($1,$2,$3,'preview-attribution',$4,'conversion-key','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','share','PENDING',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, []any{conversionID, owner, position, trackingID}},
	}
	for _, query := range queries {
		if _, err := db.Exec(query.q, query.a...); err != nil {
			t.Fatal(err)
		}
	}
}
