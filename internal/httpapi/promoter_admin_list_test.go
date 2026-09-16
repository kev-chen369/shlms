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

type adminListFunc func(context.Context, promoter.AdminActor, promoter.ApplicationListInput) (promoter.ApplicationPage, error)

func (f adminListFunc) ListApplications(ctx context.Context, a promoter.AdminActor, in promoter.ApplicationListInput) (promoter.ApplicationPage, error) {
	return f(ctx, a, in)
}

func TestAdminListHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, query, user string
		read              bool
		err               error
		want              int
		called            bool
	}{
		{"default", "", "admin", true, nil, 200, true},
		{"filter", "?status=PENDING&limit=5", "admin", true, nil, 200, true},
		{"anonymous", "", "", false, nil, 401, false},
		{"no read permission", "", "admin", false, nil, 403, false},
		{"zero limit", "?limit=0", "admin", true, nil, 400, false},
		{"repeated", "?limit=1&limit=2", "admin", true, nil, 400, false},
		{"unknown parameter", "?userId=u1", "admin", true, nil, 400, false},
		{"invalid cursor", "?cursor=bad", "admin", true, promoter.ErrInvalidInput, 400, true},
		{"storage failure", "", "admin", true, errors.New("private db"), 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
				return promoter.AdminActor{ID: tc.user, Permissions: map[string]bool{promoter.ReadPermission: tc.read}}, nil
			}), PromoterAdminList: adminListFunc(func(ctx context.Context, a promoter.AdminActor, in promoter.ApplicationListInput) (promoter.ApplicationPage, error) {
				called = true
				if a.ID != "admin" {
					t.Fatal(a)
				}
				if tc.name == "filter" && (in.Status != promoter.Pending || in.Limit != 5) {
					t.Fatal(in)
				}
				return promoter.ApplicationPage{Items: []promoter.ApplicationListItem{}}, tc.err
			})}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/admin/v1/promoter-applications"+tc.query, nil))
			if w.Code != tc.want || called != tc.called || strings.Contains(w.Body.String(), "private") || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String(), called)
			}
		})
	}
}
