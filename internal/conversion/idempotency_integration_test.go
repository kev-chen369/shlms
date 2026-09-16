package conversion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

func integrationConversionService(db *sql.DB) Service {
	return Service{
		Eligibility: preview.PostgresEligibility{DB: db},
		Previews:    preview.NewRepository(db), Requests: Repository{DB: db},
		Requoter: requoterFunc(func(_ context.Context, product string, position preview.ChannelPosition) (preview.Quote, error) {
			expectedPosition := "position-u1"
			if product == "sku-alt" {
				expectedPosition = "position-alt"
			} else if product != "sku" {
				return preview.Quote{}, fmt.Errorf("unexpected product %q", product)
			}
			if position.AccountID != "account" || position.ExternalPositionID != expectedPosition {
				return preview.Quote{}, fmt.Errorf("unexpected channel mapping for %q", product)
			}
			return preview.Quote{ExternalProductID: product, ProductName: "product", CouponPriceMinor: 1000,
				PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: time.Now().Add(time.Minute)}, nil
		}),
	}
}

func addAlternativePosition(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version)
		VALUES('pos-alt','u1','alternative','home','ENABLED',1);
		INSERT INTO channel_positions(position_id,channel,account_id,external_position_id,status,version)
		VALUES('pos-alt','JD','account','position-alt','READY',1);
		INSERT INTO promotion_previews(id,owner_user_id,position_id,idempotency_key,request_fingerprint,scene,channel,external_product_id,product_name,currency,coupon_price_minor,promoter_estimate_minor,consumer_cashback_estimate_minor,rule_version,evidence_ref,updated_at,expires_at)
		VALUES('pv-alt','u1','pos-alt','preview-alt','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','home','JD','sku-alt','product','CNY',1000,100,50,'v1','evidence',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP + interval '10 minutes')`); err != nil {
		t.Fatal(err)
	}
}

func TestConvertConcurrentSameKeyDifferentPositions(t *testing.T) {
	db := conversionDB(t)
	addAlternativePosition(t, db)
	s := integrationConversionService(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	inputs := []ConvertInput{
		{OwnerUserID: "u1", PositionID: "pos-u1", PreviewID: "pv-u1", Scene: "home", IdempotencyKey: "shared-key"},
		{OwnerUserID: "u1", PositionID: "pos-alt", PreviewID: "pv-alt", Scene: "home", IdempotencyKey: "shared-key"},
	}
	const callers = 12
	results := make([]Record, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = s.Convert(ctx, inputs[i%2])
		}(i)
	}
	close(start)
	wg.Wait()
	var winner Record
	successes, conflicts := 0, 0
	for i, result := range results {
		if errs[i] != nil {
			if !errors.Is(errs[i], ErrIdempotencyConflict) {
				t.Fatalf("caller %d: unexpected error %v", i, errs[i])
			}
			conflicts++
			continue
		}
		successes++
		if result.PositionID != inputs[i%2].PositionID || result.PreviewID != inputs[i%2].PreviewID || result.Status != "PENDING" {
			t.Fatalf("caller %d received another request's receipt: %+v", i, result)
		}
		if winner.ID == "" {
			winner = result
		} else if winner.ID != result.ID || winner.TrackingID != result.TrackingID {
			t.Fatalf("multiple winning receipts: %+v / %+v", winner, result)
		}
	}
	if successes != 6 || conflicts != 6 {
		t.Fatalf("successes=%d conflicts=%d, want 6 each", successes, conflicts)
	}
	var position, previewID, product, owner string
	if err := db.QueryRow(`SELECT cr.position_id,cr.preview_id,t.external_product_id,t.user_id
		FROM promotion_conversion_requests cr JOIN tracking_records t ON t.id=cr.tracking_id`).Scan(&position, &previewID, &product, &owner); err != nil {
		t.Fatal(err)
	}
	expectedProduct := "sku"
	if winner.PositionID == "pos-alt" {
		expectedProduct = "sku-alt"
	}
	if position != winner.PositionID || previewID != winner.PreviewID || product != expectedProduct || owner != "u1" {
		t.Fatalf("stored attribution mismatch: %s %s %s %s", position, previewID, product, owner)
	}
	// Alter only the position to verify it independently participates in the
	// service fingerprint, even when preview, scene and key remain unchanged.
	changed := inputs[0]
	if winner.PositionID == "pos-alt" {
		changed = inputs[1]
	}
	if changed.PositionID == "pos-u1" {
		changed.PositionID = "pos-alt"
	} else {
		changed.PositionID = "pos-u1"
	}
	if _, err := s.Convert(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("same key with changed position: %v, want ErrIdempotencyConflict", err)
	}
	var trackingCount, requestCount int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM tracking_records),(SELECT count(*) FROM promotion_conversion_requests)`).Scan(&trackingCount, &requestCount); err != nil || trackingCount != 1 || requestCount != 1 {
		t.Fatal(trackingCount, requestCount, err)
	}
}

func TestConvertRejectsCrossOwnerPositionAndPreviewWithoutWrites(t *testing.T) {
	for _, tc := range []struct {
		name, position, previewID string
		want                      error
	}{
		{"foreign position", "pos-u2", "pv-u2", ErrPosition},
		{"foreign preview", "pos-u1", "pv-u2", ErrPreview},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := conversionDB(t)
			s := integrationConversionService(db)
			_, err := s.Convert(context.Background(), ConvertInput{OwnerUserID: "u1", PositionID: tc.position,
				PreviewID: tc.previewID, Scene: "home", IdempotencyKey: "owner-isolation"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			assertNoConversionWrites(t, db)
		})
	}
}
