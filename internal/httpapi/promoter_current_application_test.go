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

type currentReaderFunc func(context.Context, string) (promoter.CurrentApplication, error)

func (f currentReaderFunc) GetCurrentApplication(ctx context.Context, id string) (promoter.CurrentApplication, error) {
	return f(ctx, id)
}

func TestCurrentApplicationHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, user, owner string
		status            promoter.Status
		err               error
		want              int
	}{
		{"unauthorized", "", "u1", promoter.Pending, nil, 401},
		{"missing", "u1", "u1", promoter.Pending, promoter.ErrNotFound, 404},
		{"failed", "u1", "u1", promoter.Pending, errors.New("private db"), 503},
		{"other owner", "u1", "u2", promoter.Pending, nil, 503},
		{"pending", "u1", "u1", promoter.Pending, nil, 200},
		{"approved", "u1", "u1", promoter.Enabled, nil, 200},
		{"rejected", "u1", "u1", promoter.Rejected, nil, 200},
		{"disabled", "u1", "u1", promoter.Disabled, nil, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return tc.user, nil }), PromoterCurrentApplication: currentReaderFunc(func(ctx context.Context, id string) (promoter.CurrentApplication, error) {
				if tc.user == "" || id != "u1" {
					t.Fatal("untrusted identity", id)
				}
				return promoter.CurrentApplication{Application: promoter.Application{ID: "app-1", UserID: tc.owner}, Status: tc.status, Reason: "review note"}, tc.err
			})}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/api/v1/promoter/applications/current?userId=u2", nil)
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.want || strings.Contains(w.Body.String(), "private") || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.want == 200 {
				var body struct {
					Data struct {
						CanReapply bool
						Reason     string
						Status     promoter.Status
					}
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Data.CanReapply != (tc.status == promoter.Rejected) || body.Data.Status != tc.status {
					t.Fatal(body)
				}
				if tc.status != promoter.Rejected && tc.status != promoter.Disabled && body.Data.Reason != "" {
					t.Fatal("internal note exposed")
				}
			}
		})
	}
}
