package material

import (
	"encoding/base64"
	"strings"
	"testing"
)

func queryFixture() Query {
	return Query{OwnerID: "11111111-1111-1111-1111-111111111111", Scope: Context{Platform: "JD", Type: "PRODUCT", CityCode: "310100", Business: "retail", Terminal: "H5"}, Limit: 20}
}

func TestQueryCursorRoundTrip(t *testing.T) {
	q := queryFixture()
	after, err := ParseQuery(q)
	if err != nil || after != "" {
		t.Fatalf("first page: %q %v", after, err)
	}
	id := "22222222-2222-2222-2222-222222222222"
	cursor, err := EncodeCursor(q, id)
	if err != nil {
		t.Fatal(err)
	}
	q.Cursor = cursor
	q.Limit = 100
	after, err = ParseQuery(q)
	if err != nil || after != id {
		t.Fatalf("next page: %q %v", after, err)
	}
	for _, change := range []func(*Query){func(q *Query) { q.OwnerID = id }, func(q *Query) { q.Scope.Platform = "TB" }, func(q *Query) { q.Scope.Type = "ACTIVITY" }, func(q *Query) { q.Scope.CityCode = "110100" }, func(q *Query) { q.Scope.Business = "travel" }, func(q *Query) { q.Scope.Terminal = "WX_MINI" }} {
		other := q
		change(&other)
		if _, err := ParseQuery(other); err != ErrInvalid {
			t.Fatalf("cross scope accepted: %+v", other)
		}
	}
}

func TestQueryRejectsInvalidInput(t *testing.T) {
	for _, change := range []func(*Query){func(q *Query) { q.OwnerID = "" }, func(q *Query) { q.OwnerID = "00000000-0000-0000-0000-000000000000" }, func(q *Query) { q.Limit = 0 }, func(q *Query) { q.Limit = 101 }, func(q *Query) { q.Scope.Platform = "MT" }, func(q *Query) { q.Scope.Terminal = "APP" }, func(q *Query) { q.Scope.CityCode = " city" }, func(q *Query) { q.Scope.Business = string([]byte{255}) }} {
		q := queryFixture()
		change(&q)
		if _, err := ParseQuery(q); err != ErrInvalid {
			t.Fatalf("invalid accepted: %+v", q)
		}
	}
	for _, raw := range []string{"!", strings.Repeat("a", 1025), base64.RawURLEncoding.EncodeToString([]byte(`{}`)), base64.RawURLEncoding.EncodeToString([]byte(`{"version":2}`))} {
		q := queryFixture()
		q.Cursor = raw
		if _, err := ParseQuery(q); err != ErrInvalid {
			t.Fatalf("bad cursor accepted: %s", raw)
		}
	}
	if _, err := EncodeCursor(queryFixture(), "not-a-uuid"); err != ErrInvalid {
		t.Fatal("invalid boundary accepted")
	}
}

func TestCursorRejectsNonCanonicalAndUnknownJSON(t *testing.T) {
	q := queryFixture()
	cursor, err := EncodeCursor(q, "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(cursor)
	for _, bad := range []string{string(raw) + `{}`, strings.TrimSuffix(string(raw), "}") + `,"secret":"x"}`, strings.Replace(string(raw), `"version":1`, `"version":2`, 1), strings.TrimSuffix(string(raw), "}") + `,"version":1}`, " " + string(raw), strings.TrimSuffix(strings.Replace(string(raw), `"version":1,`, "", 1), "}") + `,"version":1}`, strings.Replace(string(raw), `"afterId":"22222222-2222-2222-2222-222222222222"`, `"afterId":"00000000-0000-0000-0000-000000000000"`, 1)} {
		q.Cursor = base64.RawURLEncoding.EncodeToString([]byte(bad))
		if _, err := ParseQuery(q); err != ErrInvalid {
			t.Fatalf("invalid JSON accepted: %s", bad)
		}
	}
	q.Cursor = cursor + "\n"
	if _, err := ParseQuery(q); err != ErrInvalid {
		t.Fatal("noncanonical cursor accepted")
	}
}

func TestQueryFieldBoundsAndEncoderValidation(t *testing.T) {
	q := queryFixture()
	q.Scope.CityCode = strings.Repeat("c", 32)
	q.Scope.Business = strings.Repeat("b", 40)
	q.Limit = 1
	id := "22222222-2222-2222-2222-222222222222"
	cursor, err := EncodeCursor(q, id)
	if err != nil || len(cursor) > 1024 {
		t.Fatalf("valid maximum fields: %v", err)
	}
	q.Cursor = cursor
	if after, err := ParseQuery(q); err != nil || after != id {
		t.Fatalf("maximum roundtrip: %q %v", after, err)
	}
	for _, change := range []func(*Query){func(q *Query) { q.Scope.CityCode += "c" }, func(q *Query) { q.Scope.Business += "b" }, func(q *Query) { q.Scope.CityCode = "a\nb" }, func(q *Query) { q.OwnerID = strings.ToUpper("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa") }, func(q *Query) { q.Cursor = "invalid" }} {
		bad := q
		change(&bad)
		if _, err := EncodeCursor(bad, id); err != ErrInvalid {
			t.Fatalf("encoder accepted invalid query: %+v", bad)
		}
	}
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "22222222-2222-2222-2222-222222222222 "} {
		if _, err := EncodeCursor(q, id); err != ErrInvalid {
			t.Fatal("invalid boundary accepted")
		}
	}
}
