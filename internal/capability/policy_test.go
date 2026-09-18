package capability

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)

func approved() Record {
	return Record{Key: Key{Platform: "JD", MaterialType: "PRODUCT", Kind: "PRODUCT_LINK", MediaID: "media", PositionID: "position", Scene: "home", Terminal: "H5"}, Status: "READY", Evidence: Evidence{OwnerID: "reviewer", MediaApprovalRef: "media-proof", SourceApprovalRef: "source-proof", InterfaceVersion: "v1", RealCallEvidenceRef: "call-proof", VerifiedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}}
}

func TestScopedReady(t *testing.T) {
	record := approved()
	if got := Evaluate(record, record.Key, now); !got.Allowed || got.Reason != "READY" {
		t.Fatalf("complete scoped READY rejected: %+v", got)
	}
	changes := []struct {
		name   string
		change func(*Key)
	}{
		{"platform", func(k *Key) { k.Platform = "TB" }}, {"type", func(k *Key) { k.MaterialType = "ACTIVITY"; k.Kind = "ACTIVITY_LINK" }}, {"kind", func(k *Key) { k.Kind = "PRODUCT_PREVIEW" }},
		{"media", func(k *Key) { k.MediaID = "other" }}, {"position", func(k *Key) { k.PositionID = "other" }}, {"scene", func(k *Key) { k.Scene = "other" }}, {"terminal", func(k *Key) { k.Terminal = "WX_MINI" }},
		{"city", func(k *Key) { k.CityCode = "110100" }}, {"business", func(k *Key) { k.Business = "food" }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			key := record.Key
			tc.change(&key)
			if got := Evaluate(record, key, now); got.Allowed || got.Reason != "SCOPE_MISMATCH" {
				t.Fatalf("cross-scope allowed or misclassified: %+v", got)
			}
		})
	}
}

func TestDefaultAndStatusClosed(t *testing.T) {
	if got := Evaluate(Record{}, Key{}, now); got.Allowed {
		t.Fatal("zero declaration allowed")
	}
	for _, status := range []string{"UNCONFIGURED", "PENDING_VERIFICATION", "SUSPENDED", "", "enabled"} {
		record := approved()
		record.Status = status
		if got := Evaluate(record, record.Key, now); got.Allowed {
			t.Fatalf("status %q allowed", status)
		}
	}
}

func TestReadyNeedsCompleteCurrentEvidence(t *testing.T) {
	changes := []struct {
		name   string
		change func(*Record)
	}{
		{"owner", func(r *Record) { r.Evidence.OwnerID = "" }}, {"media approval", func(r *Record) { r.Evidence.MediaApprovalRef = "" }}, {"source approval", func(r *Record) { r.Evidence.SourceApprovalRef = "" }},
		{"version", func(r *Record) { r.Evidence.InterfaceVersion = "" }}, {"real call", func(r *Record) { r.Evidence.RealCallEvidenceRef = "" }},
		{"control", func(r *Record) { r.Evidence.RealCallEvidenceRef = "secret\nproof" }}, {"space", func(r *Record) { r.Evidence.OwnerID = " reviewer" }},
		{"future", func(r *Record) { r.Evidence.VerifiedAt = now.Add(time.Second) }}, {"expired", func(r *Record) { r.Evidence.ExpiresAt = now.Add(-time.Second) }},
		{"expiry boundary", func(r *Record) { r.Evidence.ExpiresAt = now }}, {"zero verified", func(r *Record) { r.Evidence.VerifiedAt = time.Time{} }}, {"zero expiry", func(r *Record) { r.Evidence.ExpiresAt = time.Time{} }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			record := approved()
			tc.change(&record)
			if got := Evaluate(record, record.Key, now); got.Allowed {
				t.Fatal("invalid evidence allowed")
			}
		})
	}
	record := approved()
	if got := Evaluate(record, record.Key, time.Time{}); got.Allowed {
		t.Fatal("missing trusted clock allowed")
	}
}

func TestInvalidKindCannotBecomeReady(t *testing.T) {
	for _, change := range []func(*Key){func(k *Key) { k.Platform = "PDD" }, func(k *Key) { k.Kind = "PRODUCT_LINK"; k.MaterialType = "ACTIVITY" }, func(k *Key) { k.Kind = "ACTIVITY_LINK" }, func(k *Key) { k.Kind = "UNKNOWN" }, func(k *Key) { k.Terminal = "UNKNOWN" }, func(k *Key) { k.PositionID = "" }} {
		record := approved()
		change(&record.Key)
		if got := Evaluate(record, record.Key, now); got.Allowed {
			t.Fatal("invalid declaration allowed")
		}
	}
}

func TestIndependentPlatformKinds(t *testing.T) {
	cases := []struct {
		platform, material string
		allowed            []string
	}{
		{"JD", "PRODUCT", []string{"CATALOG", "PRODUCT_PREVIEW", "PRODUCT_LINK", "RESULT_LOOKUP", "ORDER_ATTRIBUTION"}},
		{"JD", "ACTIVITY", []string{"CATALOG", "ACTIVITY_LINK", "RESULT_LOOKUP", "ORDER_ATTRIBUTION"}},
		{"TB", "PRODUCT", []string{"CATALOG", "PRODUCT_PREVIEW", "PRODUCT_LINK", "RESULT_LOOKUP", "ORDER_ATTRIBUTION"}},
		{"TB", "ACTIVITY", []string{"CATALOG", "ACTIVITY_LINK", "RESULT_LOOKUP", "ORDER_ATTRIBUTION"}},
		{"MT", "PRODUCT", nil},
		{"MT", "ACTIVITY", []string{"CATALOG", "ACTIVITY_LINK", "RESULT_LOOKUP", "ORDER_ATTRIBUTION"}},
	}
	for _, tc := range cases {
		for _, kind := range []string{"CATALOG", "PRODUCT_PREVIEW", "PRODUCT_LINK", "ACTIVITY_LINK", "RESULT_LOOKUP", "ORDER_ATTRIBUTION"} {
			record := approved()
			record.Key.Platform = tc.platform
			record.Key.MaterialType = tc.material
			record.Key.Kind = kind
			want := slices.Contains(tc.allowed, kind)
			if got := Evaluate(record, record.Key, now); got.Allowed != want {
				t.Fatalf("%s/%s/%s: %+v want allowed=%v", tc.platform, tc.material, kind, got, want)
			}
		}
	}
}

func TestBoundsAndSafeProjection(t *testing.T) {
	for _, change := range []func(*Record){func(r *Record) { r.Key.MediaID = strings.Repeat("x", 129) }, func(r *Record) { r.Key.Scene = strings.Repeat("x", 81) }, func(r *Record) { r.Key.CityCode = strings.Repeat("x", 33) }, func(r *Record) { r.Key.Business = strings.Repeat("x", 41) }, func(r *Record) { r.Key.PositionID = "bad\x00id" }, func(r *Record) { r.Evidence.OwnerID = string([]byte{0xff}) }, func(r *Record) { r.Evidence.RealCallEvidenceRef = strings.Repeat("x", 129) }} {
		record := approved()
		change(&record)
		if got := Evaluate(record, record.Key, now); got.Allowed {
			t.Fatal("invalid or overlong identifier accepted")
		}
	}
	record := approved()
	record.Evidence.RealCallEvidenceRef = "private-audit-reference"
	encoded, err := json.Marshal(Evaluate(record, record.Key, now))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"allowed":true,"reason":"READY"}` {
		t.Fatalf("decision exposes internal fields: %s", encoded)
	}
	record.Evidence.VerifiedAt = now
	if !Evaluate(record, record.Key, now).Allowed {
		t.Fatal("verification start boundary rejected")
	}
}
