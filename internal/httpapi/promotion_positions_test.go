package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type positionManagerStub struct {
	change func(context.Context, string, promoter.PositionCommand) (promoter.Position, error)
	list   func(context.Context, string, promoter.PositionListInput) (promoter.PositionPage, error)
}

func (s positionManagerStub) ChangePosition(ctx context.Context, id string, c promoter.PositionCommand) (promoter.Position, error) {
	return s.change(ctx, id, c)
}
func (s positionManagerStub) ListPositions(ctx context.Context, id string, in promoter.PositionListInput) (promoter.PositionPage, error) {
	return s.list(ctx, id, in)
}

func TestPositionHTTPWrites(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, action string
		want                             int
	}{
		{"create", "POST", "/api/v1/promotion-positions", `{"name":"name","scene":"group","isDefault":true}`, promoter.PositionCreate, 200},
		{"edit", "PATCH", "/api/v1/promotion-positions/p1", `{"name":"name","scene":"group","version":1}`, promoter.PositionEdit, 200},
		{"default", "POST", "/api/v1/promotion-positions/p1/default", `{"version":1}`, promoter.PositionDefault, 200},
		{"disable", "POST", "/api/v1/promotion-positions/p1/disable", `{"version":1}`, promoter.PositionDisable, 200},
		{"client owner", "POST", "/api/v1/promotion-positions", `{"name":"name","scene":"group","ownerUserId":"other"}`, "", 400},
		{"client readiness", "POST", "/api/v1/promotion-positions", `{"name":"name","scene":"group","readiness":"READY"}`, "", 400},
		{"no version", "PATCH", "/api/v1/promotion-positions/p1", `{"name":"name","scene":"group"}`, "", 400},
		{"default via edit", "PATCH", "/api/v1/promotion-positions/p1", `{"name":"name","scene":"group","version":1,"isDefault":true}`, "", 400},
		{"multiple values", "POST", "/api/v1/promotion-positions", `{} {}`, "", 400},
		{"large", "POST", "/api/v1/promotion-positions", `{"name":"` + strings.Repeat("x", 5000) + `"}`, "", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return "u1", nil }), Positions: positionManagerStub{change: func(ctx context.Context, id string, c promoter.PositionCommand) (promoter.Position, error) {
				called = true
				if id != "u1" || c.Action != tc.action || c.IdempotencyKey != "key" {
					t.Fatal(id, c)
				}
				if c.Action != promoter.PositionCreate && c.ID != "p1" {
					t.Fatal(c)
				}
				return promoter.Position{ID: "p1", OwnerUserID: "u1", Status: promoter.Enabled, Version: 1}, nil
			}}}
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", "key")
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.want || called != (tc.want == 200) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String(), called)
			}
			if tc.want == 200 {
				var b struct {
					Data struct {
						CanConvert bool
						Channels   []struct{ Readiness string }
					}
				}
				if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
					t.Fatal(err)
				}
				if b.Data.CanConvert || len(b.Data.Channels) != 1 || b.Data.Channels[0].Readiness != "WAITING_CONFIGURATION" {
					t.Fatal("false channel readiness", w.Body.String())
				}
			}
		})
	}
}

func TestPositionHTTPListAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, query, user, owner string
		want                     int
		called                   bool
	}{
		{"own list", "?status=ENABLED&limit=2", "u1", "u1", 200, true},
		{"anonymous", "", "", "u1", 401, false},
		{"forged user", "?userId=u2", "u1", "u1", 400, false},
		{"repeat limit", "?limit=1&limit=2", "u1", "u1", 400, false},
		{"invalid limit", "?limit=101", "u1", "u1", 400, false},
		{"wrong owner from dependency", "", "u1", "u2", 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return tc.user, nil }), Positions: positionManagerStub{list: func(ctx context.Context, id string, in promoter.PositionListInput) (promoter.PositionPage, error) {
				called = true
				if id != "u1" {
					t.Fatal(id)
				}
				return promoter.PositionPage{Items: []promoter.Position{{ID: "p1", OwnerUserID: tc.owner}}}, nil
			}}}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotion-positions"+tc.query, nil))
			if w.Code != tc.want || called != tc.called {
				t.Fatal(w.Code, w.Body.String(), called)
			}
		})
	}
}

func TestPositionHTTPErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{promoter.ErrInvalidInput, 400, "INVALID_REQUEST"}, {promoter.ErrNotEnabled, 403, "PROMOTER_NOT_ENABLED"},
		{promoter.ErrNotFound, 404, "POSITION_NOT_FOUND"}, {promoter.ErrConflict, 409, "VERSION_CONFLICT"},
		{promoter.ErrTransition, 409, "POSITION_DISABLED"}, {promoter.ErrIdempotencyConflict, 409, "IDEMPOTENCY_CONFLICT"},
		{errors.New("private database"), 503, "POSITIONS_UNAVAILABLE"},
	} {
		d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return "u1", nil }), Positions: positionManagerStub{change: func(context.Context, string, promoter.PositionCommand) (promoter.Position, error) {
			return promoter.Position{}, tc.err
		}}}
		r := httptest.NewRequest("POST", "/api/v1/promotion-positions/p1/default", strings.NewReader(`{"version":1}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "key")
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, r)
		if w.Code != tc.want || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
