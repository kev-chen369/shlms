package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/coupon"
)

type outboundHTTPStub struct {
	input  coupon.OutboundInput
	result coupon.OutboundResult
	err    error
}

func (s *outboundHTTPStub) Prepare(_ context.Context, in coupon.OutboundInput) (coupon.OutboundResult, error) {
	s.input = in
	return s.result, s.err
}

func TestCouponOutboundRouteIsGatedAndBoundToIdentity(t *testing.T) {
	stub := &outboundHTTPStub{result: coupon.OutboundResult{ID: "out-1", CouponID: "MT:c1", Platform: "MT", TargetURL: "https://approved.example/a"}}
	d := Dependencies{Users: fixedUserResolver{userID: "u1"}}
	request := func(body, key string) *http.Request {
		r := httptest.NewRequest("POST", "/api/v1/coupons/MT:c1/outbound", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		return r
	}
	w := httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, request(`{}`, "key-1"))
	if w.Code != 404 {
		t.Fatal("route available without resolver", w.Code)
	}
	d.Outbound = stub
	dNoUser := Dependencies{Outbound: stub}
	w = httptest.NewRecorder()
	NewRouterWithDependencies(dNoUser).ServeHTTP(w, request(`{}`, "key-1"))
	if w.Code != 404 {
		t.Fatal("route available without identity", w.Code)
	}
	for _, tc := range []struct {
		body, key string
		want      int
	}{
		{`{"terminal":"H5","entryPoint":"coupon-detail","cityCode":"110100"}`, "key-1", 200},
		{`{"ownerKey":"forged","terminal":"H5","entryPoint":"detail"}`, "key-1", 400},
		{`{"targetUrl":"https://evil.example","terminal":"H5","entryPoint":"detail"}`, "key-1", 400},
		{`null`, "key-1", 400},
		{`{}`, "key-1", 400},
		{`{}`, "", 400},
		{`{} {}`, "key-1", 400},
	} {
		w = httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, request(tc.body, tc.key))
		if w.Code != tc.want {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if tc.want == 200 && (!strings.Contains(w.Body.String(), "READY_TO_OPEN") || strings.Contains(w.Body.String(), "CLAIMED")) {
			t.Fatal("wrong outbound state", w.Body.String())
		}
	}
	if stub.input.OwnerKey != "user:u1" || stub.input.CouponID != "MT:c1" || stub.input.CityCode != "110100" {
		t.Fatal(stub.input)
	}
	stub.err = coupon.ErrOutboundConflict
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, request(`{"terminal":"H5","entryPoint":"detail"}`, "key-1"))
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	stub.err = errors.New("raw channel secret")
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, request(`{"terminal":"H5","entryPoint":"detail"}`, "key-1"))
	if w.Code != 503 || strings.Contains(w.Body.String(), "raw channel secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	d.Users = fixedUserResolver{userID: ""}
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, request(`{"terminal":"H5","entryPoint":"detail"}`, "key-1"))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
