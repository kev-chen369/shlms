package conversion

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestShareArtifactRequiresOwnedSuccessfulSafeLink(t *testing.T) {
	db := conversionDB(t)
	_, err := db.Exec(`INSERT INTO tracking_records(id,idempotency_key,user_id,channel,external_product_id,source,created_at)
		VALUES('share-tracking','share-key','u1','JD','sku','PROMOTION_CENTER',CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO promotion_conversion_requests
		(id,owner_user_id,position_id,preview_id,tracking_id,idempotency_key,request_fingerprint,scene,status,channel_request_id,link_url,created_at,updated_at)
		VALUES('share-request','u1','pos-u1','pv-u1','share-tracking','share-convert-key','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','home','SUCCEEDED','share-request','https://approved.example/item?track=1',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	reader := ShareReader{DB: db}
	link, err := reader.GetArtifact(context.Background(), "u1", "share-request", "link")
	if err != nil || link.Content != "https://approved.example/item?track=1" {
		t.Fatal(link, err)
	}
	text, err := reader.GetArtifact(context.Background(), "u1", "share-request", "text")
	if err != nil || !strings.Contains(text.Content, "product\nhttps://approved.example/item?track=1") || strings.Contains(text.Content, "个人收益") || strings.Contains(text.Content, "返现") {
		t.Fatal(text, err)
	}
	if _, err := reader.GetArtifact(context.Background(), "u2", "share-request", "link"); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-user share leaked", err)
	}
	if _, err := reader.GetArtifact(context.Background(), "u1", "share-request", "poster"); !errors.Is(err, ErrInvalid) {
		t.Fatal("poster opened", err)
	}
	_, err = db.Exec(`UPDATE promotion_conversion_requests SET status='FAILED_FINAL',link_url=NULL WHERE id='share-request'`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.GetArtifact(context.Background(), "u1", "share-request", "link"); !errors.Is(err, ErrShareNotReady) {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE promotion_conversion_requests SET status='SUCCEEDED',link_url='http://evil.example' WHERE id='share-request'`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.GetArtifact(context.Background(), "u1", "share-request", "link"); !errors.Is(err, ErrShareNotReady) {
		t.Fatal("unsafe link exposed", err)
	}
}
