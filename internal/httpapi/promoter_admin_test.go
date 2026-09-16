package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type adminResolverFunc func(*http.Request) (promoter.AdminActor, error)

func (f adminResolverFunc) ResolveAdmin(r *http.Request) (promoter.AdminActor, error) { return f(r) }

type adminExecutorFunc func(context.Context, promoter.AdminActor, promoter.AdminCommand) (promoter.Profile, error)

func (f adminExecutorFunc) Execute(ctx context.Context, a promoter.AdminActor, c promoter.AdminCommand) (promoter.Profile, error) {
	return f(ctx, a, c)
}

func adminRequest(path, body string) *http.Request {
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "key")
	return r
}

func TestAdminHTTPInputAndAuthority(t *testing.T) {
	valid := `{"approve":true,"reason":"approved","version":1}`
	for _, tc := range []struct {
		name, body, path, actor string
		permissions             map[string]bool
		want                    int
	}{
		{"review", valid, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.ReviewPermission: true}, 200},
		{"disable", `{"reason":"policy","version":2}`, "/admin/v1/promoters/u1/disable", "admin", map[string]bool{promoter.DisablePermission: true}, 200},
		{"no identity", valid, "/admin/v1/promoter-applications/a1/review", "", nil, 401},
		{"no permission", valid, "/admin/v1/promoter-applications/a1/review", "admin", nil, 403},
		{"wrong permission", valid, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.DisablePermission: true}, 403},
		{"no explicit choice", `{"reason":"reason","version":1}`, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.ReviewPermission: true}, 400},
		{"no version", `{"approve":true,"reason":"reason"}`, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.ReviewPermission: true}, 400},
		{"client actor", `{"actorId":"forged","approve":true,"reason":"reason","version":1}`, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.ReviewPermission: true}, 400},
		{"multiple JSON", valid + ` {}`, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.ReviewPermission: true}, 400},
		{"oversize", `{"reason":"` + strings.Repeat("x", 5000) + `"}`, "/admin/v1/promoter-applications/a1/review", "admin", map[string]bool{promoter.ReviewPermission: true}, 413},
		{"disable approve", valid, "/admin/v1/promoters/u1/disable", "admin", map[string]bool{promoter.DisablePermission: true}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
				return promoter.AdminActor{ID: tc.actor, Permissions: tc.permissions}, nil
			}), PromoterAdmin: adminExecutorFunc(func(ctx context.Context, a promoter.AdminActor, c promoter.AdminCommand) (promoter.Profile, error) {
				called = true
				if a.ID != "admin" || c.IdempotencyKey != "key" {
					t.Fatal(a, c)
				}
				if (c.Action == promoter.ReviewAction && c.TargetID != "a1") || (c.Action == promoter.DisableAction && c.TargetID != "u1") {
					t.Fatal(c)
				}
				return promoter.Profile{UserID: "u1", ApplicationID: "a1", Status: promoter.Enabled, Version: 2}, nil
			})}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, adminRequest(tc.path, tc.body))
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			if called != (tc.want == 200) {
				t.Fatal("unexpected service invocation")
			}
		})
	}
}

func TestAdminHTTPErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{promoter.ErrForbidden, 403, "FORBIDDEN"}, {promoter.ErrNotFound, 404, "PROMOTER_NOT_FOUND"},
		{promoter.ErrInvalidInput, 400, "INVALID_REQUEST"}, {promoter.ErrIdempotencyConflict, 409, "IDEMPOTENCY_CONFLICT"},
		{promoter.ErrConflict, 409, "VERSION_CONFLICT"}, {promoter.ErrTransition, 409, "INVALID_TRANSITION"},
		{errors.New("private database"), 503, "PROMOTER_UNAVAILABLE"},
	} {
		d := Dependencies{Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
			return promoter.AdminActor{ID: "admin", Permissions: map[string]bool{promoter.ReviewPermission: true}}, nil
		}), PromoterAdmin: adminExecutorFunc(func(context.Context, promoter.AdminActor, promoter.AdminCommand) (promoter.Profile, error) {
			return promoter.Profile{}, tc.err
		})}
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, adminRequest("/admin/v1/promoter-applications/a1/review", `{"approve":false,"reason":"reason","version":1}`))
		if w.Code != tc.want || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestAdminRoutesClosedWithoutDependencies(t *testing.T) {
	for _, d := range []Dependencies{{}, {Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
		t.Fatal("must not be called")
		return promoter.AdminActor{}, nil
	})}, {Users: identityFunc(func(*http.Request) (string, error) { return "admin", nil })}} {
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, adminRequest("/admin/v1/promoter-applications/a1/review", `{}`))
		if w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
}
