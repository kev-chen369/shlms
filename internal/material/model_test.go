package material

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)

func fixture() Record {
	return Record{ID: "11111111-1111-4111-8111-111111111111", Platform: "JD", Type: "PRODUCT", ExternalMaterialID: "123", CanonicalURL: "https://item.jd.com/123.html", Title: "测试商品", Status: "ACTIVE", EndsAt: now.Add(time.Hour), SourceUpdatedAt: now.Add(-time.Minute), RuleVersion: "v1", EvidenceRef: "private-proof", Region: Region{Mode: "NATIONWIDE"}, Terminals: []string{"H5"}}
}
func context() Context { return Context{Platform: "JD", Type: "PRODUCT", Terminal: "H5"} }

func TestProductAndActivityCards(t *testing.T) {
	record := fixture()
	card, decision := CardFor(record, context(), now)
	if !decision.Available || card.ID != record.ID || card.Title != "测试商品" {
		t.Fatalf("product hidden: %+v %+v", card, decision)
	}
	record.Platform = "MT"
	record.Type = "ACTIVITY"
	record.ExternalMaterialID = "activity-not-a-sku"
	record.StartsAt = now
	ctx := Context{Platform: "MT", Type: "ACTIVITY", Terminal: "H5"}
	if card, decision := CardFor(record, ctx, now); !decision.Available || card.Type != "ACTIVITY" {
		t.Fatalf("price-free activity hidden: %+v", decision)
	}
}

func TestTemporalAndStatusBoundaries(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*Record)
	}{
		{"draft", "NOT_ACTIVE", func(r *Record) { r.Status = "DRAFT" }}, {"suspended", "NOT_ACTIVE", func(r *Record) { r.Status = "SUSPENDED" }}, {"removed", "NOT_ACTIVE", func(r *Record) { r.Status = "REMOVED" }},
		{"not started", "NOT_STARTED", func(r *Record) { r.StartsAt = now.Add(time.Second) }}, {"expiry boundary", "EXPIRED", func(r *Record) { r.EndsAt = now }},
		{"future source", "SOURCE_NOT_CURRENT", func(r *Record) { r.SourceUpdatedAt = now.Add(time.Second) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := fixture()
			tc.change(&r)
			card, d := CardFor(r, context(), now)
			if d.Available || d.Reason != tc.want || card.ID != "" {
				t.Fatalf("want %s with zero card, got %+v %+v", tc.want, card, d)
			}
		})
	}
}

func TestScopeFiltering(t *testing.T) {
	record := fixture()
	record.Region = Region{Mode: "CITIES", CityCodes: []string{"110100", "310100"}}
	record.Business = "food"
	record.Terminals = []string{"WX_MINI"}
	ctx := Context{Platform: "JD", Type: "PRODUCT", Terminal: "WX_MINI", CityCode: "110100", Business: "food"}
	if _, d := CardFor(record, ctx, now); !d.Available {
		t.Fatal("approved scope hidden")
	}
	for _, tc := range []struct {
		name, want string
		change     func(*Context)
	}{
		{"platform", "SCOPE_MISMATCH", func(c *Context) { c.Platform = "TB" }}, {"type", "SCOPE_MISMATCH", func(c *Context) { c.Type = "ACTIVITY" }},
		{"city", "REGION_MISMATCH", func(c *Context) { c.CityCode = "440100" }}, {"missing city", "REGION_MISMATCH", func(c *Context) { c.CityCode = "" }},
		{"business", "BUSINESS_MISMATCH", func(c *Context) { c.Business = "hotel" }}, {"terminal", "TERMINAL_MISMATCH", func(c *Context) { c.Terminal = "H5" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := ctx
			tc.change(&next)
			if card, d := CardFor(record, next, now); d.Available || d.Reason != tc.want || card.ID != "" {
				t.Fatalf("cross-scope result %+v", d)
			}
		})
	}
	record.Region = Region{Mode: "NATIONWIDE"}
	record.Business = ""
	if _, d := CardFor(record, ctx, now); !d.Available {
		t.Fatal("explicit nationwide/unrestricted business scope hidden")
	}
}

func TestInvalidMaterial(t *testing.T) {
	changes := []func(*Record){func(r *Record) { r.ID = "external-id" }, func(r *Record) { r.Platform = "PDD" }, func(r *Record) { r.Platform = "MT" }, func(r *Record) { r.Type = "UNKNOWN" }, func(r *Record) { r.Title = "" }, func(r *Record) { r.EvidenceRef = "" }, func(r *Record) { r.RuleVersion = "" }, func(r *Record) { r.ExternalMaterialID = "" }, func(r *Record) { r.CanonicalURL = "http://example.com" }, func(r *Record) { r.CanonicalURL = "https://user:password@example.com" }, func(r *Record) { r.EndsAt = time.Time{} }, func(r *Record) { r.SourceUpdatedAt = time.Time{} }, func(r *Record) { r.StartsAt = r.EndsAt }, func(r *Record) { r.Type = "ACTIVITY" }, func(r *Record) { r.Region = Region{Mode: "CITIES"} }, func(r *Record) { r.Region.CityCodes = []string{"110100"} }, func(r *Record) { r.Terminals = nil }, func(r *Record) { r.Terminals = []string{"H5", "H5"} }, func(r *Record) { r.Status = "UNKNOWN" }, func(r *Record) { r.Title = "secret\nraw" }, func(r *Record) { r.Title = strings.Repeat("x", 257) }}
	for i, change := range changes {
		r := fixture()
		change(&r)
		if Validate(r) == nil {
			t.Fatalf("invalid case %d accepted", i)
		}
		if card, d := CardFor(r, context(), now); d.Available || card.ID != "" {
			t.Fatalf("invalid case %d leaked card", i)
		}
	}
	if _, d := CardFor(fixture(), Context{}, now); d.Available || d.Reason != "INVALID_CONTEXT" {
		t.Fatal("missing context allowed")
	}
	if _, d := CardFor(fixture(), context(), time.Time{}); d.Available {
		t.Fatal("missing server clock allowed")
	}
}

func TestProjectionDoesNotExposeInternalFieldsOrAliasScope(t *testing.T) {
	r := fixture()
	r.Region = Region{Mode: "CITIES", CityCodes: []string{"110100"}}
	ctx := context()
	ctx.CityCode = "110100"
	card, d := CardFor(r, ctx, now)
	if !d.Available {
		t.Fatal(d)
	}
	encoded, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"id": true, "platform": true, "type": true, "title": true, "startsAt": true, "endsAt": true, "sourceUpdatedAt": true, "ruleVersion": true, "region": true, "business": true, "terminals": true}
	for field := range fields {
		if !allowed[field] {
			t.Fatalf("internal field %s exposed", field)
		}
	}
	if strings.Contains(string(encoded), "private-proof") || strings.Contains(string(encoded), "https://") {
		t.Fatal("secret reference or URL exposed")
	}
	card.Region.CityCodes[0] = "other"
	card.Terminals[0] = "WX_MINI"
	if r.Region.CityCodes[0] != "110100" || r.Terminals[0] != "H5" {
		t.Fatal("public card mutates internal scope")
	}
}

func TestMetadataBoundaries(t *testing.T) {
	for _, change := range []func(*Record){
		func(r *Record) { r.ID = "00000000-0000-0000-0000-000000000000" },
		func(r *Record) { r.ExternalMaterialID = strings.Repeat("x", 129) }, func(r *Record) { r.RuleVersion = strings.Repeat("x", 81) }, func(r *Record) { r.EvidenceRef = strings.Repeat("x", 129) }, func(r *Record) { r.Business = strings.Repeat("x", 41) },
		func(r *Record) { r.Title = string([]byte{0xff}) }, func(r *Record) { r.Title = " leading" },
		func(r *Record) { r.Region = Region{Mode: "UNKNOWN"} }, func(r *Record) { r.Region = Region{Mode: "CITIES", CityCodes: []string{"110100", "110100"}} }, func(r *Record) { r.Region = Region{Mode: "CITIES", CityCodes: []string{strings.Repeat("x", 33)}} },
		func(r *Record) { r.Terminals = []string{"UNKNOWN"} }, func(r *Record) { r.Terminals = []string{"H5", "WX_MINI", "H5"} },
		func(r *Record) { r.CanonicalURL = "/relative" }, func(r *Record) { r.CanonicalURL = "https://example.com/a b" }, func(r *Record) { r.CanonicalURL = "https://example.com/\\path" },
	} {
		r := fixture()
		change(&r)
		if Validate(r) == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
	r := fixture()
	r.ExternalMaterialID = strings.Repeat("x", 128)
	r.Title = strings.Repeat("x", 256)
	r.RuleVersion = strings.Repeat("x", 80)
	r.EvidenceRef = strings.Repeat("x", 128)
	r.Business = strings.Repeat("x", 40)
	r.SourceUpdatedAt = now
	ctx := context()
	ctx.Business = r.Business
	if _, d := CardFor(r, ctx, now); !d.Available {
		t.Fatalf("valid max lengths/source boundary rejected: %+v", d)
	}
	for _, ctx := range []Context{{Platform: "JD", Type: "PRODUCT", Terminal: "UNKNOWN"}, {Platform: "JD", Type: "PRODUCT", Terminal: "H5", CityCode: " bad"}, {Platform: "JD", Type: "PRODUCT", Terminal: "H5", Business: "bad\ncontext"}} {
		if _, d := CardFor(fixture(), ctx, now); d.Available || d.Reason != "INVALID_CONTEXT" {
			t.Fatal("invalid request context accepted")
		}
	}
}

func TestNonJSONTimestampRejectedBeforeProjection(t *testing.T) {
	r := fixture()
	r.EndsAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if Validate(r) == nil {
		t.Fatal("unserializable timestamp accepted as public card metadata")
	}
	if card, d := CardFor(r, context(), now); d.Available || card.ID != "" {
		t.Fatal("invalid timestamp exposed")
	}
}
