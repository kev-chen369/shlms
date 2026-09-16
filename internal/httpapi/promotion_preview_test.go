package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

type previewFunc func(context.Context, preview.Request) (preview.Snapshot, error)

func (f previewFunc) Create(ctx context.Context, r preview.Request) (preview.Snapshot, error) {
	return f(ctx, r)
}

func TestPreviewHTTPContractAndErrors(t *testing.T) {
	base := `{"input":"https://approved.example/item","positionId":"p1","scene":"home"}`
	for _, tc := range []struct {
		name, body, key, contentType string
		identityErr, serviceErr      error
		status                       int
		called                       bool
	}{
		{name: "success", body: base, key: "key", contentType: "application/json", status: 200, called: true},
		{name: "unauthorized", body: base, key: "key", contentType: "application/json", identityErr: errors.New("no token"), status: 401},
		{name: "wrong media", body: base, key: "key", contentType: "text/plain", status: 415},
		{name: "missing key", body: base, contentType: "application/json", status: 400},
		{name: "owner injection", body: `{"input":"x","positionId":"p1","scene":"home","ownerUserId":"u2"}`, key: "key", contentType: "application/json", status: 400},
		{name: "price injection", body: `{"input":"x","positionId":"p1","scene":"home","couponPriceMinor":1}`, key: "key", contentType: "application/json", status: 400},
		{name: "multiple JSON", body: base + base, key: "key", contentType: "application/json", status: 400},
		{name: "too large", body: `{"input":"` + strings.Repeat("a", 9000) + `"}`, key: "key", contentType: "application/json", status: 413},
		{name: "not enabled", body: base, key: "key", contentType: "application/json", serviceErr: preview.ErrNotEnabled, status: 403, called: true},
		{name: "other position", body: base, key: "key", contentType: "application/json", serviceErr: preview.ErrPosition, status: 404, called: true},
		{name: "not ready", body: base, key: "key", contentType: "application/json", serviceErr: preview.ErrNotReady, status: 409, called: true},
		{name: "different request", body: base, key: "key", contentType: "application/json", serviceErr: preview.ErrIdempotencyConflict, status: 409, called: true},
		{name: "channel missing", body: base, key: "key", contentType: "application/json", serviceErr: preview.ErrUnavailable, status: 503, called: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{
				Users: identityFunc(func(*http.Request) (string, error) { return "u1", tc.identityErr }),
				Preview: previewFunc(func(_ context.Context, in preview.Request) (preview.Snapshot, error) {
					called = true
					if in.OwnerUserID != "u1" || in.PositionID != "p1" || in.IdempotencyKey != "key" || in.Scene != "home" {
						t.Fatal(in)
					}
					if tc.serviceErr != nil {
						return preview.Snapshot{}, tc.serviceErr
					}
					return preview.Snapshot{ID: "pv1", OwnerUserID: "u1", PositionID: "p1", Channel: "JD", Currency: "CNY",
						ProductName: "商品", ExternalProductID: "sku", UpdatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute)}, nil
				}),
			}
			r := httptest.NewRequest("POST", "/api/v1/promotions/preview", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.status || called != tc.called || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, called, w.Body.String())
			}
			if tc.status == 200 && (!strings.Contains(w.Body.String(), `"previewId":"pv1"`) || strings.Contains(w.Body.String(), "evidence")) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestPreviewRouteRequiresDependencies(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/promotions/preview", nil)
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
