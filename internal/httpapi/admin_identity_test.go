package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/auth"
	"github.com/kev-chen369/shlms/internal/promoter"
)

func TestAdminAuthorizationOutageReturns503(t *testing.T) {
	d := Dependencies{
		Admins: adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) { return promoter.AdminActor{}, auth.ErrUnavailable }),
		PromoterAdmin: adminExecutorFunc(func(context.Context, promoter.AdminActor, promoter.AdminCommand) (promoter.Profile, error) {
			t.Fatal("admin action reached during outage")
			return promoter.Profile{}, nil
		}),
		PromoterAdminList: adminListFunc(func(context.Context, promoter.AdminActor, promoter.ApplicationListInput) (promoter.ApplicationPage, error) {
			t.Fatal("admin list reached during outage")
			return promoter.ApplicationPage{}, nil
		}),
		ChannelPositions: configureChannelFunc(func(context.Context, promoter.AdminActor, promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error) {
			t.Fatal("channel config reached during outage")
			return promoter.ChannelPosition{}, nil
		}),
	}
	for _, path := range []string{"/admin/v1/promoter-applications/app-1/review", "/admin/v1/promoter-applications", "/admin/v1/promotion-positions/p1/channels/JD"} {
		method := "POST"
		if path == "/admin/v1/promoter-applications" {
			method = "GET"
		}
		if strings.Contains(path, "channels") {
			method = "PUT"
		}
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 503 || !strings.Contains(w.Body.String(), "ADMIN_AUTH_UNAVAILABLE") {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
}
