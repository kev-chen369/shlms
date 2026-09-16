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

type profileReaderFunc func(context.Context, string) (promoter.Profile, error)

func (f profileReaderFunc) GetProfile(ctx context.Context, id string) (promoter.Profile, error) {
	return f(ctx, id)
}

type identityFunc func(*http.Request) (string, error)

func (f identityFunc) ResolveUserID(r *http.Request) (string, error) { return f(r) }

func TestPromoterProfileUsesAuthenticatedIdentity(t *testing.T) {
	for _, status := range []promoter.Status{promoter.NotApplied, promoter.Pending, promoter.Enabled, promoter.Rejected, promoter.Disabled} {
		t.Run(string(status), func(t *testing.T) {
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return "u1", nil }), Promoter: profileReaderFunc(func(ctx context.Context, id string) (promoter.Profile, error) {
				if id != "u1" {
					t.Fatal("trusted client identity")
				}
				return promoter.Profile{UserID: id, Status: status, Reason: "reason", ApplicationID: "a1"}, nil
			})}
			r := httptest.NewRequest("GET", "/api/v1/promoter/profile?userId=u2", nil)
			r.Header.Set("X-User-ID", "u2")
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			var body struct {
				Data struct {
					Status       promoter.Status
					Reason       string
					Capabilities promoter.Capabilities
				}
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Data.Status != status || body.Data.Capabilities != (promoter.Profile{Status: status}).Permissions() {
				t.Fatal(body)
			}
			if status != promoter.Rejected && status != promoter.Disabled && body.Data.Reason != "" {
				t.Fatal("internal note leaked")
			}
		})
	}
}

func TestPromoterProfileFailures(t *testing.T) {
	for _, tc := range []struct {
		name, identity   string
		authErr, readErr error
		owner            string
		want             int
	}{
		{"unauthenticated", "", errors.New("private auth"), nil, "u1", 401},
		{"empty identity", " ", nil, nil, "u1", 401},
		{"repository failed", "u1", nil, errors.New("private database"), "u1", 503},
		{"wrong owner", "u1", nil, nil, "u2", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return tc.identity, tc.authErr }), Promoter: profileReaderFunc(func(context.Context, string) (promoter.Profile, error) {
				called = true
				return promoter.Profile{UserID: tc.owner, Status: promoter.Enabled}, tc.readErr
			})}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/profile", nil))
			if w.Code != tc.want || strings.Contains(w.Body.String(), "private") {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.want == 401 && called {
				t.Fatal("unauthorized repository access")
			}
		})
	}
}

func TestPromoterRouteRequiresBothDependencies(t *testing.T) {
	for _, d := range []Dependencies{{}, {Users: identityFunc(func(*http.Request) (string, error) { return "u1", nil })}, {Promoter: profileReaderFunc(func(context.Context, string) (promoter.Profile, error) {
		t.Fatal("must not be called")
		return promoter.Profile{}, nil
	})}} {
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/profile", nil))
		if w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
}
