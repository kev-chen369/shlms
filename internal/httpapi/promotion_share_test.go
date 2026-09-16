package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/conversion"
)

func TestShareArtifactsHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, query, status, owner, id, link string
		identityErr, readErr                 error
		want                                 int
	}{
		{name: "link", query: "type=link", status: "SUCCEEDED", want: 200},
		{name: "text", query: "type=text", status: "SUCCEEDED", want: 200},
		{name: "unauthorized", query: "type=link", identityErr: errors.New("private token"), want: 401},
		{name: "cross owner", query: "type=link", status: "SUCCEEDED", owner: "u2", want: 404},
		{name: "wrong record", query: "type=link", status: "SUCCEEDED", id: "cr2", want: 404},
		{name: "missing", query: "type=link", readErr: conversion.ErrNotFound, want: 404},
		{name: "invalid ID", query: "type=link", readErr: conversion.ErrInvalid, want: 400},
		{name: "database failure", query: "type=link", readErr: errors.New("private database credential"), want: 503},
		{name: "pending", query: "type=link", status: "PENDING", want: 409},
		{name: "processing", query: "type=text", status: "PROCESSING", want: 409},
		{name: "uncertain", query: "type=link", status: "FAILED_RETRYABLE", want: 409},
		{name: "failed", query: "type=text", status: "FAILED_FINAL", want: 409},
		{name: "qr", query: "type=qr", want: 422},
		{name: "poster", query: "type=poster", want: 422},
		{name: "missing type", want: 400},
		{name: "unknown type", query: "type=other", want: 400},
		{name: "duplicate type", query: "type=link&type=text", want: 400},
		{name: "owner injection", query: "type=text&ownerUserId=u2", want: 400},
		{name: "malformed query", query: "type=%zz", want: 400},
		{name: "unsafe scheme", query: "type=text", status: "SUCCEEDED", link: "javascript:private", want: 503},
		{name: "private IP", query: "type=link", status: "SUCCEEDED", link: "https://127.0.0.1/item", want: 503},
		{name: "credentials", query: "type=text", status: "SUCCEEDED", link: "https://private:secret@channel.example/item", want: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, id, link := "u1", "cr1", "https://channel.example/item"
			if tc.owner != "" {
				owner = tc.owner
			}
			if tc.id != "" {
				id = tc.id
			}
			if tc.link != "" {
				link = tc.link
			}
			d := Dependencies{
				Users: identityFunc(func(*http.Request) (string, error) { return "u1", tc.identityErr }),
				ConversionReader: conversionReadFunc(func(_ context.Context, gotOwner, gotID string) (conversion.Record, error) {
					if tc.identityErr != nil {
						t.Fatal("unauthenticated read")
					}
					if gotOwner != "u1" || gotID != "cr1" {
						t.Fatal(gotOwner, gotID)
					}
					return conversion.Record{ID: id, OwnerUserID: owner, TrackingID: "tr1", Status: tc.status, LinkURL: link,
						SchemeURL: "private-scheme", RequestFingerprint: "private-fingerprint", IdempotencyKey: "private-key"}, tc.readErr
				}),
				Conversion: conversionFunc(func(context.Context, conversion.ConvertInput) (conversion.Record, error) {
					t.Fatal("share read attempted conversion")
					return conversion.Record{}, nil
				}),
			}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotion-links/cr1/share-artifacts?"+tc.query, nil))
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "收益") {
				t.Fatal("private data leaked", w.Body.String())
			}
			if tc.want == 200 {
				var response struct {
					Data map[string]string `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				wantType, content := "link", "https://channel.example/item"
				if tc.name == "text" {
					wantType, content = "text", "万惠宝好物推荐\nhttps://channel.example/item"
				}
				data := response.Data
				if len(data) != 6 || data["linkId"] != "cr1" || data["trackingId"] != "tr1" || data["type"] != wantType || data["linkUrl"] != "https://channel.example/item" || data["content"] != content || data["templateVersion"] != "v1" {
					t.Fatal(data)
				}
			} else if strings.Contains(w.Body.String(), "channel.example") {
				t.Fatal("failure exposed link", w.Body.String())
			}
		})
	}
}

func TestShareArtifactsRequiresDependencies(t *testing.T) {
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotion-links/cr1/share-artifacts?type=link", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func historicShareDB(t *testing.T) *sql.DB {
	t.Helper()
	db := promotionCenterDB(t)
	for _, name := range []string{"000001_tracking_records", "000007_promotion_previews", "000008_promotion_conversion_requests", "000009_conversion_state"} {
		b, err := os.ReadFile("../../migrations/" + name + ".up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatal(name, err)
		}
	}
	// Synthetic historic success only: no READY configuration, Gateway or real
	// channel evidence is created. Disabled owners retain read-only history.
	if _, err := db.Exec(`
		INSERT INTO promoter_profiles(user_id,status) VALUES('u1','NOT_APPLIED'),('u2','NOT_APPLIED');
		INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status)
		VALUES('app1','u1','apply','User','home','v1',CURRENT_TIMESTAMP,'ENABLED');
		UPDATE promoter_profiles SET status='DISABLED',application_id='app1' WHERE user_id='u1';
		INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES('p1','u1','main','home','DISABLED',1);
		INSERT INTO tracking_records(id,idempotency_key,user_id,channel,external_product_id,source,created_at)
		VALUES('tr1','tracking-key','u1','JD','sku','home',CURRENT_TIMESTAMP);
		INSERT INTO promotion_previews(id,owner_user_id,position_id,idempotency_key,request_fingerprint,scene,channel,external_product_id,product_name,currency,coupon_price_minor,promoter_estimate_minor,consumer_cashback_estimate_minor,rule_version,evidence_ref,updated_at,expires_at)
		VALUES('pv1','u1','p1','preview-key',repeat('a',64),'home','JD','sku','product','CNY',1000,100,50,'v1','private-evidence',CURRENT_TIMESTAMP - interval '2 minutes',CURRENT_TIMESTAMP - interval '1 minute');
		INSERT INTO promotion_conversion_requests(id,owner_user_id,position_id,preview_id,tracking_id,idempotency_key,request_fingerprint,scene,status,channel_request_id,link_url,version,attempt_count)
		VALUES('cr1','u1','p1','pv1','tr1','convert-key',repeat('b',64),'home','SUCCEEDED','cr1','https://channel.example/item',3,1)`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestShareArtifactsPostgresReadOnlyAndOwnerIsolation(t *testing.T) {
	db := historicShareDB(t)
	userID := "u1"
	router := NewRouterWithDependencies(Dependencies{
		Users:            identityFunc(func(*http.Request) (string, error) { return userID, nil }),
		ConversionReader: conversion.ReadService{Repository: conversion.Repository{DB: db}},
	})
	for _, tc := range []struct {
		owner, kind string
		want        int
	}{
		{"u1", "link", 200}, {"u1", "text", 200}, {"u1", "text", 200}, {"u2", "link", 404},
	} {
		userID = tc.owner
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotion-links/cr1/share-artifacts?type="+tc.kind, nil))
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.want == 200 && !strings.Contains(w.Body.String(), `"trackingId":"tr1"`) {
			t.Fatal(w.Body.String())
		}
		if tc.want == 404 && strings.Contains(w.Body.String(), "channel.example") {
			t.Fatal("cross-owner leak", w.Body.String())
		}
	}
	var trackingCount, requestCount, attempts int
	var version int64
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM tracking_records),(SELECT count(*) FROM promotion_conversion_requests),attempt_count,version FROM promotion_conversion_requests WHERE id='cr1'`).Scan(&trackingCount, &requestCount, &attempts, &version); err != nil || trackingCount != 1 || requestCount != 1 || attempts != 1 || version != 3 {
		t.Fatal(trackingCount, requestCount, attempts, version, err)
	}
}
