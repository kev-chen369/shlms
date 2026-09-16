package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/conversion"
)

type conversionFunc func(context.Context, conversion.ConvertInput) (conversion.Record, error)

func (f conversionFunc) Convert(ctx context.Context, in conversion.ConvertInput) (conversion.Record, error) {
	return f(ctx, in)
}

func TestConvertHTTPContract(t *testing.T) {
	base := `{"previewId":"pv1","positionId":"p1","scene":"home"}`
	for _, tc := range []struct {
		name, body, media, key  string
		identityErr, serviceErr error
		status                  int
		called                  bool
	}{
		{name: "pending", body: base, media: "application/json", key: "key", status: 202, called: true},
		{name: "unauthorized", body: base, media: "application/json", key: "key", identityErr: errors.New("no token"), status: 401},
		{name: "media", body: base, media: "text/plain", key: "key", status: 415},
		{name: "key", body: base, media: "application/json", status: 400},
		{name: "owner injection", body: `{"previewId":"pv1","positionId":"p1","scene":"home","ownerUserId":"other"}`, media: "application/json", key: "key", status: 400},
		{name: "price injection", body: `{"previewId":"pv1","positionId":"p1","scene":"home","couponPriceMinor":1}`, media: "application/json", key: "key", status: 400},
		{name: "multiple JSON", body: base + base, media: "application/json", key: "key", status: 400},
		{name: "large", body: `{"previewId":"` + strings.Repeat("x", 5000) + `"}`, media: "application/json", key: "key", status: 413},
		{name: "not enabled", body: base, media: "application/json", key: "key", serviceErr: conversion.ErrNotEnabled, status: 403, called: true},
		{name: "not ready", body: base, media: "application/json", key: "key", serviceErr: conversion.ErrNotReady, status: 409, called: true},
		{name: "expired", body: base, media: "application/json", key: "key", serviceErr: conversion.ErrExpired, status: 409, called: true},
		{name: "changed", body: base, media: "application/json", key: "key", serviceErr: conversion.ErrPriceChanged, status: 409, called: true},
		{name: "conflict", body: base, media: "application/json", key: "key", serviceErr: conversion.ErrIdempotencyConflict, status: 409, called: true},
		{name: "unavailable", body: base, media: "application/json", key: "key", serviceErr: conversion.ErrUnavailable, status: 503, called: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{
				Users: identityFunc(func(*http.Request) (string, error) { return "u1", tc.identityErr }),
				Conversion: conversionFunc(func(_ context.Context, in conversion.ConvertInput) (conversion.Record, error) {
					called = true
					if in.OwnerUserID != "u1" || in.PreviewID != "pv1" || in.PositionID != "p1" || in.Scene != "home" || in.IdempotencyKey != "key" {
						t.Fatal(in)
					}
					if tc.serviceErr != nil {
						return conversion.Record{}, tc.serviceErr
					}
					return conversion.Record{ID: "cr1", TrackingID: "tr1", OwnerUserID: "u1", Status: "PENDING"}, nil
				}),
			}
			r := httptest.NewRequest("POST", "/api/v1/promotions/convert", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.status || called != tc.called || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, called, w.Body.String())
			}
			if tc.status == 202 && (!strings.Contains(w.Body.String(), `"trackingId":"tr1"`) || !strings.Contains(w.Body.String(), `"statusUrl":"/api/v1/promotions/convert/cr1"`) || strings.Contains(w.Body.String(), "linkUrl")) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestConvertRouteRequiresDependencies(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/promotions/convert", nil)
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
