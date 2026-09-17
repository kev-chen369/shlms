package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/conversion"
)

type shareEventFunc func(context.Context, conversion.ShareEventInput) (conversion.ShareRecord, error)

func (f shareEventFunc) RecordShareEvent(ctx context.Context, in conversion.ShareEventInput) (conversion.ShareRecord, error) {
	return f(ctx, in)
}

func TestShareEventsHTTP(t *testing.T) {
	base := `{"eventId":"event1","action":"copy_link","scene":"group"}`
	for _, tc := range []struct {
		name, body, media, key  string
		identityErr, serviceErr error
		want                    int
		called                  bool
	}{
		{name: "record", body: base, media: "application/json", key: "event1", want: 200, called: true},
		{name: "unauthorized", body: base, media: "application/json", key: "event1", identityErr: errors.New("private token"), want: 401},
		{name: "media", body: base, media: "text/plain", key: "event1", want: 415},
		{name: "missing key", body: base, media: "application/json", want: 400},
		{name: "different key", body: base, media: "application/json", key: "other", want: 400},
		{name: "owner injection", body: `{"eventId":"event1","action":"copy_link","scene":"group","ownerUserId":"u2"}`, media: "application/json", key: "event1", want: 400},
		{name: "delivery claim", body: `{"eventId":"event1","action":"copy_link","scene":"group","delivered":true}`, media: "application/json", key: "event1", want: 400},
		{name: "multiple JSON", body: base + base, media: "application/json", key: "event1", want: 400},
		{name: "large", body: `{"scene":"` + strings.Repeat("x", 5000) + `"}`, media: "application/json", key: "event1", want: 413},
		{name: "invalid", body: base, media: "application/json", key: "event1", serviceErr: conversion.ErrInvalid, want: 400, called: true},
		{name: "not found", body: base, media: "application/json", key: "event1", serviceErr: conversion.ErrNotFound, want: 404, called: true},
		{name: "pending", body: base, media: "application/json", key: "event1", serviceErr: conversion.ErrStateConflict, want: 409, called: true},
		{name: "conflict", body: base, media: "application/json", key: "event1", serviceErr: conversion.ErrIdempotencyConflict, want: 409, called: true},
		{name: "database failure", body: base, media: "application/json", key: "event1", serviceErr: errors.New("private credential"), want: 503, called: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return "u1", tc.identityErr }),
				ShareEvents: shareEventFunc(func(_ context.Context, in conversion.ShareEventInput) (conversion.ShareRecord, error) {
					called = true
					if in.OwnerUserID != "u1" || in.LinkID != "cr1" || in.EventID != "event1" || in.Action != "copy_link" || in.Scene != "group" {
						t.Fatal(in)
					}
					return conversion.ShareRecord{EventID: "event1", LinkID: "cr1", TrackingID: "tr1", Action: "copy_link", Scene: "group", RecordedAt: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)}, tc.serviceErr
				})}
			r := httptest.NewRequest("POST", "/api/v1/promotion-links/cr1/share-events", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.want || called != tc.called || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, called, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "delivered") || strings.Contains(w.Body.String(), "收益") {
				t.Fatal(w.Body.String())
			}
			if tc.want == 200 && (!strings.Contains(w.Body.String(), `"eventId":"event1"`) || !strings.Contains(w.Body.String(), `"trackingId":"tr1"`) || !strings.Contains(w.Body.String(), `"recordedAt":"2026-09-17T00:00:00Z"`)) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestShareEventsRequiresDependencies(t *testing.T) {
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/promotion-links/cr1/share-events", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestShareEventsPostgresReplayConflictAndNoNewTracking(t *testing.T) {
	db := historicShareDB(t)
	b, err := os.ReadFile("../../migrations/000010_promotion_share_events.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatal(err)
	}
	userID := "u1"
	router := NewRouterWithDependencies(Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return userID, nil }), ShareEvents: conversion.Repository{DB: db}})
	var recordedAt string
	for _, tc := range []struct {
		owner, action string
		want          int
	}{
		{"u1", "copy_link", 200}, {"u1", "copy_link", 200}, {"u1", "copy_text", 409}, {"u2", "copy_link", 404}, {"u1", "delivered", 400},
	} {
		userID = tc.owner
		r := httptest.NewRequest("POST", "/api/v1/promotion-links/cr1/share-events", strings.NewReader(`{"eventId":"event1","action":"`+tc.action+`","scene":"group"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "event1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.want == 200 {
			var result struct {
				Data map[string]string `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Data) != 6 || result.Data["trackingId"] != "tr1" || result.Data["linkId"] != "cr1" || result.Data["eventId"] != "event1" || result.Data["action"] != "copy_link" || result.Data["scene"] != "group" || result.Data["recordedAt"] == "" {
				t.Fatal(result)
			}
			if recordedAt != "" && recordedAt != result.Data["recordedAt"] {
				t.Fatal("replay changed receipt", result)
			}
			recordedAt = result.Data["recordedAt"]
		}
	}
	var events, tracking, requests, attempts int
	var version int64
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM promotion_share_events),(SELECT count(*) FROM tracking_records),(SELECT count(*) FROM promotion_conversion_requests),attempt_count,version FROM promotion_conversion_requests WHERE id='cr1'`).Scan(&events, &tracking, &requests, &attempts, &version); err != nil || events != 1 || tracking != 1 || requests != 1 || attempts != 1 || version != 3 {
		t.Fatal(events, tracking, requests, attempts, version, err)
	}
}
