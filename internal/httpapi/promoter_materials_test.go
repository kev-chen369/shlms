package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/capability"
	"github.com/kev-chen369/shlms/internal/material"
)

const materialOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const materialID = "00000000-0000-4000-8000-000000000001"
const materialQuery = "platform=JD&type=PRODUCT&terminal=H5&positionId=p1&scene=home&cityCode=310100&business=food"

type materialReaderStub struct {
	input        material.ReadInput
	cursor, id   string
	limit, calls int
	page         material.Page
	detail       material.Detail
	err          error
}

func (s *materialReaderStub) List(_ context.Context, in material.ReadInput, cursor string, limit int) (material.Page, error) {
	s.input, s.cursor, s.limit = in, cursor, limit
	s.calls++
	return s.page, s.err
}
func (s *materialReaderStub) Get(_ context.Context, in material.ReadInput, id string) (material.Detail, error) {
	s.input, s.id = in, id
	s.calls++
	return s.detail, s.err
}

func materialCard() material.Card {
	return material.Card{ID: materialID, Platform: "JD", Type: "PRODUCT", Title: "catalog item", EndsAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), SourceUpdatedAt: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), RuleVersion: "v1", Region: material.Region{Mode: "NATIONWIDE"}, Terminals: []string{"H5"}}
}

// Catches trusting caller identity, losing filters/pagination or wrong JSON data.
func TestPromoterMaterialRoutesBindIdentityAndReadScope(t *testing.T) {
	card := materialCard()
	stub := &materialReaderStub{page: material.Page{Items: []material.Card{card}, Capability: capability.Decision{Allowed: true, Reason: "READY"}}, detail: material.Detail{Item: &card, Capability: capability.Decision{Allowed: true, Reason: "READY"}, Availability: material.Decision{Available: true, Reason: "AVAILABLE"}}}
	router := NewRouterWithDependencies(Dependencies{Users: fixedUserResolver{userID: materialOwner}, Materials: stub})
	in := material.ReadInput{OwnerID: materialOwner, Scope: material.Context{Platform: "JD", Type: "PRODUCT", CityCode: "310100", Business: "food", Terminal: "H5"}, PositionID: "p1", Scene: "home"}
	cursor, err := material.EncodeCursor(material.Query{OwnerID: materialOwner, Scope: in.Scope, Limit: 2}, materialID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		detail bool
		limit  int
		cursor string
	}{
		{"/api/v1/promoter/materials?" + materialQuery, false, 20, ""},
		{"/api/v1/promoter/materials?" + materialQuery + "&limit=2&cursor=" + url.QueryEscape(cursor), false, 2, cursor},
		{"/api/v1/promoter/materials/" + materialID + "?" + materialQuery, true, 0, ""},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", tc.path, nil)
		r.Header.Set("Authorization", "Bearer private-token")
		router.ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || !reflect.DeepEqual(stub.input, in) {
			t.Fatal(w.Code, w.Body.String(), stub.input)
		}
		var envelope struct {
			Code int
			Data json.RawMessage
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != 0 {
			t.Fatal(err, w.Body.String())
		}
		if tc.detail {
			var data material.Detail
			if err := json.Unmarshal(envelope.Data, &data); err != nil || data.Item == nil || data.Item.ID != materialID || !data.Availability.Available || stub.id != materialID {
				t.Fatal(data, err)
			}
		} else {
			var data material.Page
			if err := json.Unmarshal(envelope.Data, &data); err != nil || len(data.Items) != 1 || data.Items[0].Title != "catalog item" || !data.Capability.Allowed || stub.limit != tc.limit || stub.cursor != tc.cursor {
				t.Fatal(data, err, stub)
			}
		}
		for _, forbidden := range []string{"private-token", "mediaId", "evidenceRef", "canonicalUrl", "canGenerate", "price"} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatalf("leaked %s", forbidden)
			}
		}
	}
}

// Catches registering read/write APIs without identity/service or calling the
// reader before authentication; malformed queries must not disclose auth state.
func TestPromoterMaterialRoutesFailClosedWithoutIdentity(t *testing.T) {
	stub := &materialReaderStub{}
	for _, d := range []Dependencies{{}, {Users: fixedUserResolver{userID: materialOwner}}, {Materials: stub}} {
		for _, path := range []string{"/api/v1/promoter/materials", "/api/v1/promoter/materials/" + materialID} {
			w := httptest.NewRecorder()
			NewRouterWithDependencies(d).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 404 {
				t.Fatal(w.Code)
			}
		}
	}
	for _, path := range []string{"/api/v1/promoter/materials?ownerId=other", "/api/v1/promoter/materials/" + materialID + "?mediaId=secret"} {
		w := httptest.NewRecorder()
		NewRouterWithDependencies(Dependencies{Users: fixedUserResolver{}, Materials: stub}).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 || w.Header().Get("Cache-Control") != "no-store" || stub.calls != 0 {
			t.Fatal(w.Code, stub.calls, w.Body.String())
		}
	}
	wAuth := httptest.NewRecorder()
	NewRouterWithDependencies(Dependencies{Users: identityFunc(func(*http.Request) (string, error) { return materialOwner, errors.New("private auth error") }), Materials: stub}).ServeHTTP(wAuth, httptest.NewRequest("GET", "/api/v1/promoter/materials?"+materialQuery, nil))
	if wAuth.Code != 401 || stub.calls != 0 || strings.Contains(wAuth.Body.String(), "private auth error") {
		t.Fatal(wAuth.Code, wAuth.Body.String(), stub.calls)
	}
	w := httptest.NewRecorder()
	NewRouterWithDependencies(Dependencies{Users: fixedUserResolver{userID: materialOwner}, Materials: stub}).ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/promoter/materials", nil))
	if w.Code != 405 || stub.calls != 0 {
		t.Fatal(w.Code, stub.calls)
	}
}

// Catches unknown/duplicated injection parameters, accepting wrong contexts or
// detail pagination, and allowing the service to be called for malformed input.
func TestPromoterMaterialRoutesStrictQueries(t *testing.T) {
	stub := &materialReaderStub{}
	router := NewRouterWithDependencies(Dependencies{Users: fixedUserResolver{userID: materialOwner}, Materials: stub})
	for _, query := range []string{
		materialQuery + "&ownerId=other", materialQuery + "&%6fwnerId=other", materialQuery + "&mediaId=media", materialQuery + "&status=READY", materialQuery + "&now=2026-09-19", materialQuery + "&kind=PRODUCT_LINK", materialQuery + "&platform=TB", materialQuery + "&limit=0", materialQuery + "&limit=101", materialQuery + "&limit=bad", materialQuery + "&cursor=bad", materialQuery + "&limit=1&limit=2", materialQuery + "&x=%zz", materialQuery + ";bad=1",
		"platform=MT&type=PRODUCT&terminal=H5&positionId=p1&scene=home", "platform=JD&type=PRODUCT&terminal=WEB&positionId=p1&scene=home", "platform=JD&type=PRODUCT&terminal=H5&positionId=p1", "platform=JD&type=PRODUCT&terminal=H5&scene=home", strings.Replace(materialQuery, "scene=home", "scene=%20home", 1), strings.Replace(materialQuery, "positionId=p1", "positionId=%00p1", 1), strings.Replace(materialQuery, "business=food", "business="+strings.Repeat("x", 41), 1),
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/materials?"+query, nil))
		if w.Code != 400 || w.Header().Get("Cache-Control") != "no-store" || stub.calls != 0 {
			t.Fatal(query, w.Code, w.Body.String(), stub.calls)
		}
	}
	for _, query := range []string{materialQuery + "&limit=1", materialQuery + "&cursor=", materialQuery + "&ownerId=other", materialQuery + "&platform=TB"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/promoter/materials/"+materialID+"?"+query, nil))
		if w.Code != 400 || stub.calls != 0 {
			t.Fatal(query, w.Code, stub.calls)
		}
	}
}

// Catches converting a safe capability/material refusal to success data or
// exposing a storage failure/partial result through either HTTP read endpoint.
func TestPromoterMaterialRoutesDenialsAndSafeErrors(t *testing.T) {
	stub := &materialReaderStub{page: material.Page{Items: []material.Card{}, Capability: capability.Decision{Reason: "UNCONFIGURED"}}, detail: material.Detail{Capability: capability.Decision{Allowed: true, Reason: "READY"}, Availability: material.Decision{Reason: "EXPIRED"}}}
	router := NewRouterWithDependencies(Dependencies{Users: fixedUserResolver{userID: materialOwner}, Materials: stub})
	for _, base := range []string{"/api/v1/promoter/materials", "/api/v1/promoter/materials/" + materialID} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", base+"?"+materialQuery, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), `"item":`) || strings.Contains(w.Body.String(), materialID) {
			t.Fatal(w.Code, w.Body.String())
		}
		if base == "/api/v1/promoter/materials" {
			if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"allowed":false`) || !strings.Contains(w.Body.String(), `"reason":"UNCONFIGURED"`) {
				t.Fatal(w.Body.String())
			}
		} else if !strings.Contains(w.Body.String(), `"available":false`) || !strings.Contains(w.Body.String(), `"reason":"EXPIRED"`) || !strings.Contains(w.Body.String(), `"allowed":true`) {
			t.Fatal(w.Body.String())
		}
		for _, tc := range []struct {
			err    error
			status int
			code   string
		}{{material.ErrInvalid, 400, "INVALID_REQUEST"}, {errors.New("private SQL media-proof"), 503, "MATERIALS_UNAVAILABLE"}, {context.Canceled, 503, "MATERIALS_UNAVAILABLE"}} {
			card := materialCard()
			stub.page.Items, stub.detail.Item = []material.Card{card}, &card
			stub.err = tc.err
			w = httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", base+"?"+materialQuery, nil))
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private SQL") || strings.Contains(w.Body.String(), "media-proof") || !strings.Contains(w.Body.String(), `"data":null`) || strings.Contains(w.Body.String(), materialID) {
				t.Fatal(w.Code, w.Body.String())
			}
		}
		stub.err = nil
		stub.page.Items, stub.detail.Item = []material.Card{}, nil
	}
}
