package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/order"
)

type orderReaderStub struct {
	filter    order.OrderFilter
	owner, id string
	err       error
}

func (s *orderReaderStub) ListOwned(_ context.Context, in order.OrderFilter) (order.OrderPage, error) {
	s.filter = in
	return order.OrderPage{Items: []order.OrderSummary{}}, s.err
}
func (s *orderReaderStub) GetOwned(_ context.Context, owner, id string) (order.OrderDetail, error) {
	s.owner, s.id = owner, id
	return order.OrderDetail{OrderSummary: order.OrderSummary{ID: id}}, s.err
}

func TestPromoterOrderRoutesRequireIdentityAndValidateQuery(t *testing.T) {
	stub := &orderReaderStub{}
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/orders", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	d := Dependencies{Users: fixedUserResolver{userID: "owner-1"}, Orders: stub}
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/v1/promoter/orders?channel=JD&positionId=p1&orderStatus=PAID&limit=2", 200},
		{"/api/v1/promoter/orders?limit=0", 400},
		{"/api/v1/promoter/orders?channel=JD&channel=MT", 400},
		{"/api/v1/promoter/orders?earningStatus=AVAILABLE", 400},
		{"/api/v1/promoter/orders?from=bad", 400},
	} {
		w = httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.want {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	if stub.filter.OwnerUserID != "owner-1" || stub.filter.Channel != "JD" || stub.filter.PositionID != "p1" || stub.filter.Status != "PAID" {
		t.Fatal(stub.filter)
	}
	d.Users = fixedUserResolver{userID: ""}
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/orders", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	d.Users = fixedUserResolver{userID: "owner-1"}
	stub.err = order.ErrNotFound
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/orders/some-id", nil))
	if w.Code != 404 || stub.owner != "owner-1" {
		t.Fatal(w.Code, stub.owner)
	}
	stub.err = errors.New("secret database detail")
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/orders", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret database detail") {
		t.Fatal(w.Code, w.Body.String())
	}
}
