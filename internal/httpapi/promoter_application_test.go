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

type applicantFunc func(context.Context, promoter.SubmitInput) (promoter.Application, error)

func (f applicantFunc) SubmitApplication(ctx context.Context, in promoter.SubmitInput) (promoter.Application, error) {
	return f(ctx, in)
}

func TestApplicationHTTPValidation(t *testing.T) {
	valid := `{"displayName":"name","scene":"group","agreementVersion":"v1","agreed":true}`
	for _, tc := range []struct {
		name, body, media, key, user string
		want                         int
	}{
		{"ok", valid, "application/json", "key", "u1", 200},
		{"unauthorized", valid, "application/json", "key", "", 401},
		{"media", valid, "text/plain", "key", "u1", 415},
		{"key", valid, "application/json", "", "u1", 400},
		{"unknown", `{"userId":"other"}`, "application/json", "key", "u1", 400},
		{"trailing", valid + ` {}`, "application/json", "key", "u1", 400},
		{"malformed", `{`, "application/json", "key", "u1", 400},
		{"large", `{"displayName":"` + strings.Repeat("a", 5000) + `"}`, "application/json", "key", "u1", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return tc.user, nil }), PromoterApplications: applicantFunc(func(ctx context.Context, in promoter.SubmitInput) (promoter.Application, error) {
				called = true
				if in.UserID != "u1" || !in.Agreed || in.IdempotencyKey != "key" {
					t.Fatal(in)
				}
				return promoter.Application{ID: "app-1", UserID: in.UserID}, nil
			})}
			r := httptest.NewRequest("POST", "/api/v1/promoter/applications?userId=other", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.media)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, r)
			if w.Code != tc.want || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.want != 200 && called {
				t.Fatal("invalid request called service")
			}
		})
	}
}

func TestApplicationHTTPErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{promoter.ErrAgreement, 422, "AGREEMENT_REQUIRED"}, {promoter.ErrInvalidInput, 400, "INVALID_REQUEST"},
		{promoter.ErrIdempotencyConflict, 409, "IDEMPOTENCY_CONFLICT"}, {promoter.ErrTransition, 409, "APPLICATION_NOT_ALLOWED"},
		{errors.New("private database password"), 503, "PROMOTER_UNAVAILABLE"},
	} {
		d := Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return "u1", nil }), PromoterApplications: applicantFunc(func(context.Context, promoter.SubmitInput) (promoter.Application, error) {
			return promoter.Application{}, tc.err
		})}
		r := httptest.NewRequest("POST", "/api/v1/promoter/applications", strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "key")
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, r)
		if w.Code != tc.want || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
