package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/coupon"
)

type claimHTTPStub struct {
	input  coupon.ClaimInput
	result coupon.Claim
	err    error
}

func (s *claimHTTPStub) Create(_ context.Context, in coupon.ClaimInput) (coupon.Claim, error) {
	s.input = in
	return s.result, s.err
}
func (s *claimHTTPStub) Get(_ context.Context, owner, id string) (coupon.Claim, error) {
	return s.result, s.err
}

func TestCouponClaimRoutesFailClosedAndProtectIdentity(t *testing.T) {
	stub := &claimHTTPStub{result: coupon.Claim{ID: "claim-1", OwnerUserID: "u1", CouponID: "JD:c1", Status: "QUERY_REQUIRED", EvidenceRef: "secret-proof"}}
	base := Dependencies{Users: fixedUserResolver{userID: "u1"}, ClaimReader: stub}
	w := httptest.NewRecorder()
	NewRouterWithDependencies(base).ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/coupons/JD:c1/claims", strings.NewReader(`{}`)))
	if w.Code != 404 {
		t.Fatal("POST registered without adapter", w.Code)
	}
	w = httptest.NewRecorder()
	NewRouterWithDependencies(base).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupon-claims/claim-1", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret-proof") {
		t.Fatal(w.Code, w.Body.String())
	}
	base.Claims = stub
	for _, tc := range []struct {
		body, key string
		want      int
	}{
		{`{"cityCode":"110100","business":"food"}`, "key-1", 202},
		{`{"ownerUserId":"u2"}`, "key-1", 400},
		{`{}`, "", 400},
		{`{} {}`, "key-1", 400},
	} {
		r := httptest.NewRequest("POST", "/api/v1/coupons/JD:c1/claims", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.key != "" {
			r.Header.Set("Idempotency-Key", tc.key)
		}
		w = httptest.NewRecorder()
		NewRouterWithDependencies(base).ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	if stub.input.OwnerUserID != "u1" || stub.input.CouponID != "JD:c1" {
		t.Fatal(stub.input)
	}
	stub.err = coupon.ErrNotClaimable
	r := httptest.NewRequest("POST", "/api/v1/coupons/JD:c1/claims", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", "key-1")
	w = httptest.NewRecorder()
	NewRouterWithDependencies(base).ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	stub.err = errors.New("channel secret")
	w = httptest.NewRecorder()
	NewRouterWithDependencies(base).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/coupon-claims/claim-1", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "channel secret") {
		t.Fatal(w.Code, w.Body.String())
	}
}
