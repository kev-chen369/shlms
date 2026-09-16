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

type configureChannelFunc func(context.Context, promoter.AdminActor, promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error)

func (f configureChannelFunc) Configure(ctx context.Context, actor promoter.AdminActor, in promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error) {
	return f(ctx, actor, in)
}

func channelConfigRequest(path, body string) *http.Request {
	r := httptest.NewRequest("PUT", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "configuration-key")
	return r
}

func TestChannelConfigurationAuthorityAndValidation(t *testing.T) {
	valid := `{"accountId":"account-1","externalPositionId":"jd-position-1","version":0}`
	for _, tc := range []struct {
		name, path, body, actor string
		authorized              bool
		want                    int
	}{
		{"ok", "/admin/v1/promotion-positions/p1/channels/JD", valid, "admin", true, 200},
		{"anonymous", "/admin/v1/promotion-positions/p1/channels/JD", valid, "", false, 401},
		{"forbidden", "/admin/v1/promotion-positions/p1/channels/JD", valid, "admin", false, 403},
		{"unsupported", "/admin/v1/promotion-positions/p1/channels/PDD", valid, "admin", true, 422},
		{"missing version", "/admin/v1/promotion-positions/p1/channels/JD", `{"accountId":"a","externalPositionId":"b"}`, "admin", true, 400},
		{"forged status", "/admin/v1/promotion-positions/p1/channels/JD", `{"accountId":"a","externalPositionId":"b","version":0,"status":"READY"}`, "admin", true, 400},
		{"forged owner", "/admin/v1/promotion-positions/p1/channels/JD", `{"accountId":"a","externalPositionId":"b","version":0,"ownerUserId":"other"}`, "admin", true, 400},
		{"trailing JSON", "/admin/v1/promotion-positions/p1/channels/JD", valid + ` {}`, "admin", true, 400},
		{"oversize", "/admin/v1/promotion-positions/p1/channels/JD", `{"accountId":"` + strings.Repeat("x", 5000) + `"}`, "admin", true, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			d := Dependencies{Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
				return promoter.AdminActor{ID: tc.actor, Permissions: map[string]bool{promoter.ConfigureChannelPermission: tc.authorized}}, nil
			}), ChannelPositions: configureChannelFunc(func(ctx context.Context, actor promoter.AdminActor, in promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error) {
				called = true
				if actor.ID != "admin" || in.PositionID != "p1" || in.Channel != "JD" || in.AccountID != "account-1" || in.ExternalPositionID != "jd-position-1" || in.ExpectedVersion != 0 || in.IdempotencyKey != "configuration-key" {
					t.Fatal(actor, in)
				}
				return promoter.ChannelPosition{PositionID: in.PositionID, Channel: "JD", Status: "PENDING_VERIFICATION", Version: 1}, nil
			})}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, channelConfigRequest(tc.path, tc.body))
			if w.Code != tc.want || called != (tc.want == 200) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Code, w.Body.String(), called)
			}
		})
	}
}

func TestChannelConfigurationErrorsAreSafe(t *testing.T) {
	valid := `{"accountId":"account-1","externalPositionId":"jd-position-1","version":0}`
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{promoter.ErrForbidden, 403, "FORBIDDEN"}, {promoter.ErrInvalidInput, 400, "INVALID_REQUEST"},
		{promoter.ErrNotFound, 404, "POSITION_NOT_FOUND"}, {promoter.ErrNotEnabled, 403, "PROMOTER_NOT_ENABLED"},
		{promoter.ErrTransition, 409, "POSITION_DISABLED"}, {promoter.ErrConflict, 409, "VERSION_CONFLICT"},
		{promoter.ErrIdempotencyConflict, 409, "IDEMPOTENCY_CONFLICT"}, {promoter.ErrExternalPositionConflict, 409, "EXTERNAL_POSITION_CONFLICT"},
		{errors.New("secret upstream diagnostic"), 503, "CHANNEL_POSITION_UNAVAILABLE"},
	} {
		d := Dependencies{Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
			return promoter.AdminActor{ID: "admin", Permissions: map[string]bool{promoter.ConfigureChannelPermission: true}}, nil
		}), ChannelPositions: configureChannelFunc(func(context.Context, promoter.AdminActor, promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error) {
			return promoter.ChannelPosition{}, tc.err
		})}
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, channelConfigRequest("/admin/v1/promotion-positions/p1/channels/JD", valid))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "secret upstream diagnostic") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestChannelConfigurationFailsClosedWithoutAdminDependencies(t *testing.T) {
	for _, d := range []Dependencies{{}, {Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
		t.Fatal("admin resolver should not run")
		return promoter.AdminActor{}, nil
	})}, {ChannelPositions: configureChannelFunc(func(context.Context, promoter.AdminActor, promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error) {
		t.Fatal("configurator should not run")
		return promoter.ChannelPosition{}, nil
	})}} {
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, channelConfigRequest("/admin/v1/promotion-positions/p1/channels/JD", `{}`))
		if w.Code != 404 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
