package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/conversion"
)

type shareReaderStub struct {
	owner, id, kind string
	err             error
}

func (s *shareReaderStub) GetArtifact(_ context.Context, owner, id, kind string) (conversion.ShareArtifact, error) {
	s.owner, s.id, s.kind = owner, id, kind
	return conversion.ShareArtifact{RequestID: id, Type: kind, Content: "https://approved.example/item"}, s.err
}

func TestShareArtifactRoute(t *testing.T) {
	stub := &shareReaderStub{}
	w := httptest.NewRecorder()
	NewRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotions/convert/c1/share-artifacts?type=link", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	d := Dependencies{Users: fixedUserResolver{userID: "u1"}, ShareArtifacts: stub}
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/v1/promotions/convert/c1/share-artifacts?type=link", 200},
		{"/api/v1/promotions/convert/c1/share-artifacts?type=text", 200},
		{"/api/v1/promotions/convert/c1/share-artifacts?type=poster", 400},
		{"/api/v1/promotions/convert/c1/share-artifacts?type=link&type=text", 400},
		{"/api/v1/promotions/convert/c1/share-artifacts", 400},
	} {
		w = httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.want {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	if stub.owner != "u1" || stub.id != "c1" {
		t.Fatal(stub)
	}
	stub.err = conversion.ErrShareNotReady
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotions/convert/c1/share-artifacts?type=link", nil))
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	stub.err = errors.New("private database detail")
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotions/convert/c1/share-artifacts?type=link", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "private database detail") {
		t.Fatal(w.Code, w.Body.String())
	}
	d.Users = fixedUserResolver{userID: ""}
	w = httptest.NewRecorder()
	NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promotions/convert/c1/share-artifacts?type=link", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
