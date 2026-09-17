package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/coupon"
)

type couponReaderStub struct {
	input coupon.ListInput
	err   error
}

func (s *couponReaderStub) List(_ context.Context, in coupon.ListInput) (coupon.Page, error) {
	s.input = in
	if s.err != nil {
		return coupon.Page{}, s.err
	}
	return coupon.Page{Items: []coupon.Item{}}, nil
}

func TestCouponListRoutesAndValidation(t *testing.T) {
	stub := &couponReaderStub{}
	handler := NewRouterWithDependencies(Dependencies{Coupons: stub})
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"?platform=JD&limit=2", 200}, {"?platform=OTHER", 200}, {"?limit=0", 400},
		{"?platform=JD&platform=TB", 400}, {"?unknown=x", 400}, {"?cursor=%zz", 400},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupons"+tc.query, nil))
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
	}
	if stub.input.Platform != "OTHER" {
		t.Fatal("unexpected last reader input", stub.input)
	}
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupons", nil))
	if w.Code != 404 {
		t.Fatal("route should not exist without catalog")
	}
	stub.err = coupon.ErrInvalid
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupons?platform=OTHER", nil))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	stub.err = errors.New("database unavailable")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupons", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "database unavailable") {
		t.Fatal(w.Code, w.Body.String())
	}
}
