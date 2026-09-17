package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/coupon"
)

type citiesStub struct {
	platform string
	err      error
}

func (s *citiesStub) Cities(_ context.Context, platform string) ([]coupon.City, error) {
	s.platform = platform
	if s.err != nil {
		return nil, s.err
	}
	return []coupon.City{{Code: "110100", Name: "北京"}}, nil
}

func TestCouponCitiesRoute(t *testing.T) {
	stub := &citiesStub{}
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupon-cities?platform=MT", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	d := Dependencies{CouponCities: stub}
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/v1/coupon-cities?platform=MT", 200},
		{"/api/v1/coupon-cities", 400},
		{"/api/v1/coupon-cities?platform=MT&platform=JD", 400},
		{"/api/v1/coupon-cities?platform=MT&cityCode=110100", 400},
	} {
		w = httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.want {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	if stub.platform != "MT" {
		t.Fatal(stub.platform)
	}
	stub.err = errors.New("secret")
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupon-cities?platform=MT", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String())
	}
}
