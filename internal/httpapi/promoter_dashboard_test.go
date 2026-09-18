package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/dashboard"
)

type dashboardStub struct {
	filter dashboard.Filter
	err    error
}

func (s *dashboardStub) Get(_ context.Context, f dashboard.Filter) (dashboard.Counts, error) {
	s.filter = f
	return dashboard.Counts{TimeZone: "Asia/Shanghai", SuccessfulLinks: 2, CopyReports: 1, AsOf: time.Now()}, s.err
}

func TestDashboardRoute(t *testing.T) {
	stub := &dashboardStub{}
	d := Dependencies{Users: fixedUserResolver{userID: "u1"}, Dashboard: stub}
	call := func(path string) (int, string) {
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w.Code, w.Body.String()
	}
	code, body := call("/api/v1/promoter/dashboard?from=2026-09-01&to=2026-09-02&channel=JD")
	if code != 200 || stub.filter.OwnerUserID != "u1" || stub.filter.From.Location().String() != "Asia/Shanghai" || stub.filter.To.Sub(stub.filter.From) != 48*time.Hour || strings.Contains(body, "conversionRate") {
		t.Fatal(code, body, stub.filter)
	}
	for _, path := range []string{"/api/v1/promoter/dashboard?from=bad", "/api/v1/promoter/dashboard?from=2026-09-03&to=2026-09-01", "/api/v1/promoter/dashboard?channel=JD&channel=TB", "/api/v1/promoter/dashboard?ownerUserId=u2"} {
		code, _ = call(path)
		if code != 400 {
			t.Fatal(path, code)
		}
	}
	stub.err = errors.New("private db detail")
	code, body = call("/api/v1/promoter/dashboard")
	if code != 503 || strings.Contains(body, "private db detail") {
		t.Fatal(code, body)
	}
	d.Users = fixedUserResolver{userID: ""}
	code, _ = call("/api/v1/promoter/dashboard")
	if code != 401 {
		t.Fatal(code)
	}
}
