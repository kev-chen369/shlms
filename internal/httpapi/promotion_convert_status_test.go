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

type conversionReadFunc func(context.Context, string, string) (conversion.Record, error)

func (f conversionReadFunc) Get(ctx context.Context, owner, id string) (conversion.Record, error) {
	return f(ctx, owner, id)
}

func TestConvertStatusHTTP(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		identityErr, readErr error
		owner, status, link  string
		want                 int
	}{
		{name: "pending", owner: "u1", status: "PENDING", want: 200},
		{name: "success", owner: "u1", status: "SUCCEEDED", link: "https://channel.example/item", want: 200},
		{name: "other owner", owner: "u2", status: "PENDING", want: 404},
		{name: "missing", readErr: conversion.ErrNotFound, want: 404},
		{name: "failure", readErr: errors.New("db down"), want: 503},
		{name: "unauthorized", identityErr: errors.New("no token"), want: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Dependencies{
				Users: identityFunc(func(*http.Request) (string, error) { return "u1", tc.identityErr }),
				ConversionReader: conversionReadFunc(func(_ context.Context, owner, id string) (conversion.Record, error) {
					if owner != "u1" || id != "cr1" {
						t.Fatal(owner, id)
					}
					if tc.readErr != nil {
						return conversion.Record{}, tc.readErr
					}
					return conversion.Record{ID: "cr1", OwnerUserID: tc.owner, TrackingID: "tr1", Status: tc.status, LinkURL: tc.link}, nil
				}),
			}
			r := httptest.NewRequest("GET", "/api/v1/promotions/convert/cr1", nil)
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.want == 200 {
				if !strings.Contains(w.Body.String(), `"statusUrl":"/api/v1/promotions/convert/cr1"`) {
					t.Fatal(w.Body.String())
				}
				if strings.Contains(w.Body.String(), "linkUrl") != (tc.status == "SUCCEEDED") {
					t.Fatal(w.Body.String())
				}
			}
		})
	}
}
