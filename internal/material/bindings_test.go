package material

import (
	stdcontext "context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const bindingJSON = `{"platform":"JD","type":"PRODUCT","terminal":"H5","scene":"home","mediaId":"test-media"}`

// Catches parsing the wrong media or treating deployment selection as a grant.
func TestParsedBindingsConsumerStillChecksCurrentEvidence(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	insertCatalogMaterial(t, db, 1, fixture())
	bindings, err := ParseCatalogBindings([]byte(`[{"platform":"JD","type":"PRODUCT","terminal":"H5","scene":"home","mediaId":"catalog-media"}]`))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewReadService(Repository{DB: db}, bindings)
	if err != nil {
		t.Fatal(err)
	}
	in := ReadInput{OwnerID: q.OwnerID, Scope: q.Scope, PositionID: key.PositionID, Scene: "home"}
	// The synthetic fixture clock is test-only; production continues time.Now.
	s.now = func() time.Time { return now }
	detail, err := s.Get(stdcontext.Background(), in, catalogID(1))
	if err != nil || detail.Item == nil || detail.Item.ID != catalogID(1) || !detail.Capability.Allowed {
		t.Fatal(detail, err)
	}
	if _, err = db.Exec(`UPDATE channel_capabilities SET status='SUSPENDED'`); err != nil {
		t.Fatal(err)
	}
	detail, err = s.Get(stdcontext.Background(), in, catalogID(1))
	if err != nil || detail.Item != nil || detail.Capability.Allowed || detail.Capability.Reason != "SUSPENDED" {
		t.Fatal(detail, err)
	}
	empty, err := ParseCatalogBindings([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	s, err = NewReadService(Repository{DB: db}, empty)
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.List(stdcontext.Background(), in, "", 1)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" || page.Capability.Allowed || page.Capability.Reason != "UNCONFIGURED" {
		t.Fatal(page, err)
	}
}

// Catches empty/default returns, case normalization and loss of configured scope.
func TestParseCatalogBindingsExactValues(t *testing.T) {
	got, err := ParseCatalogBindings([]byte(" [ " + bindingJSON + ", {\"platform\":\"MT\",\"type\":\"ACTIVITY\",\"terminal\":\"WX_MINI\",\"scene\":\"活动\",\"mediaId\":\"mt-test\"} ] \n"))
	want := []CatalogBinding{{"JD", "PRODUCT", "H5", "home", "test-media"}, {"MT", "ACTIVITY", "WX_MINI", "活动", "mt-test"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	got, err = ParseCatalogBindings([]byte("[]"))
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	// The limit is bytes, not a guessed binding count; whitespace is valid JSON.
	if _, err = ParseCatalogBindings([]byte("[]" + strings.Repeat(" ", 65534))); err != nil {
		t.Fatal(err)
	}
}

// Catches permissive JSON decoding, partial returns, grant injection and logging input.
func TestParseCatalogBindingsRejectsInvalidConfiguration(t *testing.T) {
	cases := map[string]string{
		"empty": "", "null": "null", "object": bindingJSON, "scalar": "1", "element": "[null]",
		"trailing": "[" + bindingJSON + "] []", "broken": "[" + bindingJSON + ",]",
		"unknown":           "[" + strings.Replace(bindingJSON, `"mediaId"`, `"status":"READY","mediaId"`, 1) + "]",
		"case":              "[" + strings.Replace(bindingJSON, `"platform"`, `"Platform"`, 1) + "]",
		"duplicate":         "[" + strings.Replace(bindingJSON, `"platform"`, `"platform":"TB","platform"`, 1) + "]",
		"escaped-duplicate": "[" + strings.Replace(bindingJSON, `"platform"`, `"\u0070latform":"TB","platform"`, 1) + "]",
		"missing":           "[" + strings.Replace(bindingJSON, `"scene":"home",`, "", 1) + "]",
		"number":            "[" + strings.Replace(bindingJSON, `"home"`, "123", 1) + "]",
		"null-field":        "[" + strings.Replace(bindingJSON, `"home"`, "null", 1) + "]",
		"nested":            "[" + strings.Replace(bindingJSON, `"home"`, `{"secret":"private-secret"}`, 1) + "]",
		"unsupported":       "[" + strings.Replace(bindingJSON, `"JD"`, `"MT"`, 1) + "]",
		"invalid-terminal":  "[" + strings.Replace(bindingJSON, `"H5"`, `"h5"`, 1) + "]",
		"empty-media":       "[" + strings.Replace(bindingJSON, `"test-media"`, `""`, 1) + "]",
		"space":             "[" + strings.Replace(bindingJSON, `"home"`, `" home"`, 1) + "]",
		"control":           "[" + strings.Replace(bindingJSON, `"home"`, `"ho\nme"`, 1) + "]",
		"long-media":        "[" + strings.Replace(bindingJSON, `"test-media"`, `"`+strings.Repeat("x", 129)+`"`, 1) + "]",
		"duplicate-scope":   "[" + bindingJSON + "," + strings.Replace(bindingJSON, "test-media", "private-secret", 1) + "]",
		"partial":           "[" + bindingJSON + ",{}]",
		"utf8":              "[" + strings.Replace(bindingJSON, "home", string([]byte{0xff}), 1) + "]",
		"oversized":         "[]" + strings.Repeat(" ", 65535),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseCatalogBindings([]byte(input))
			if !errors.Is(err, ErrInvalid) || got != nil {
				t.Fatal(got, err)
			}
			if strings.Contains(err.Error(), "private-secret") || strings.Contains(err.Error(), "test-media") {
				t.Fatal("configuration leaked")
			}
		})
	}
}

func TestParseCatalogBindingsRejectsUnpairedSurrogates(t *testing.T) {
	for _, value := range []string{`\ud800`, `\udfff`, `\ud800x`, `\ud800\u0041`} {
		got, err := ParseCatalogBindings([]byte("[" + strings.Replace(bindingJSON, "home", value, 1) + "]"))
		if !errors.Is(err, ErrInvalid) || got != nil {
			t.Fatalf("accepted unpaired surrogate: %v %v", got, err)
		}
	}
	for _, value := range []string{`\ud83d\ude00`, `\\ud800`} {
		got, err := ParseCatalogBindings([]byte("[" + strings.Replace(bindingJSON, "home", value, 1) + "]"))
		if err != nil || len(got) != 1 {
			t.Fatal(got, err)
		}
	}
}
