package httpapi

import (
	"context"
	"encoding/json"
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
	return dashboard.Counts{TimeZone: "Asia/Shanghai", From: f.From, ToExclusive: f.To, SuccessfulLinks: 2, CopyReports: 1, AsOf: time.Now()}, s.err
}

func TestDashboardWirePreservesNaturalDayInstants(t *testing.T) {
	for _, tc := range []struct{ day, from, to string }{
		{"2026-09-01", "2026-08-31T16:00:00Z", "2026-09-01T16:00:00Z"},
		{"1991-04-14", "1991-04-13T16:00:00Z", "1991-04-14T15:00:00Z"},
		{"1991-09-15", "1991-09-14T15:00:00Z", "1991-09-15T16:00:00Z"},
		{"1900-01-01", "1899-12-31T15:54:17Z", "1900-01-01T15:54:17Z"},
	} {
		t.Run(tc.day, func(t *testing.T) {
			stub := &dashboardStub{}
			w := httptest.NewRecorder()
			NewRouterWithDependencies(Dependencies{Users: fixedUserResolver{userID: "u1"}, Dashboard: stub}).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/dashboard?from="+tc.day+"&to="+tc.day, nil))
			var response struct {
				Data struct{ TimeZone, From, ToExclusive, AsOf string }
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || response.Data.TimeZone != "Asia/Shanghai" || response.Data.From != tc.from || response.Data.ToExclusive != tc.to || !strings.HasSuffix(response.Data.AsOf, "Z") || stub.filter.From.Location().String() != "Asia/Shanghai" {
				t.Fatal(w.Code, w.Body.String(), stub.filter)
			}
		})
	}
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
