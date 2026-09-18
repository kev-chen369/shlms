// Package capability evaluates trusted server-side declarations. It does not
// verify evidence authenticity, grant promoter eligibility, or write READY.
package capability

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Key has no wildcards: an empty city/business means only that exact scope.
// Position and account/media checks must still be repeated in the generation
// transaction; this policy is not a replacement for those checks.
type Key struct {
	Platform, MaterialType, Kind         string
	MediaID, PositionID, Scene, Terminal string
	CityCode, Business                   string
}

// Evidence contains opaque references, not credentials or raw channel replies.
// Only an audited trusted repository may supply a production Record.
type Evidence struct {
	OwnerID, MediaApprovalRef, SourceApprovalRef string
	InterfaceVersion, RealCallEvidenceRef        string
	VerifiedAt, ExpiresAt                        time.Time
}
type Record struct {
	Key      Key
	Status   string
	Evidence Evidence
}
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

func bounded(value string, limit int, optional bool) bool {
	if value == "" {
		return optional
	}
	if len(value) > limit || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func validKey(key Key) bool {
	if key.Platform != "JD" && key.Platform != "TB" && key.Platform != "MT" {
		return false
	}
	if key.MaterialType != "PRODUCT" && key.MaterialType != "ACTIVITY" {
		return false
	}
	// This version only models approved MT activities. Arbitrary product links
	// need a separately approved protocol/design, never a configuration shortcut.
	if key.Platform == "MT" && key.MaterialType != "ACTIVITY" {
		return false
	}
	switch key.Kind {
	case "CATALOG", "RESULT_LOOKUP", "ORDER_ATTRIBUTION":
	case "PRODUCT_PREVIEW", "PRODUCT_LINK":
		if key.MaterialType != "PRODUCT" {
			return false
		}
	case "ACTIVITY_LINK":
		if key.MaterialType != "ACTIVITY" {
			return false
		}
	default:
		return false
	}
	return (key.Terminal == "H5" || key.Terminal == "WX_MINI") && bounded(key.MediaID, 128, false) && bounded(key.PositionID, 128, false) && bounded(key.Scene, 80, false) && bounded(key.CityCode, 32, true) && bounded(key.Business, 40, true)
}

// Evaluate is fail-closed. Declaration timestamps use [verifiedAt, expiresAt).
// now must come from the server, and record from a trusted audited repository;
// field completeness alone is not proof that a channel approved this account.
func Evaluate(record Record, requested Key, now time.Time) Decision {
	deny := func(reason string) Decision { return Decision{Reason: reason} }
	if now.IsZero() || !validKey(record.Key) || !validKey(requested) {
		return deny("INVALID_DECLARATION")
	}
	if record.Key != requested {
		return deny("SCOPE_MISMATCH")
	}
	switch record.Status {
	case "UNCONFIGURED", "PENDING_VERIFICATION", "SUSPENDED":
		return deny(record.Status)
	case "READY":
	default:
		return deny("INVALID_DECLARATION")
	}
	evidence := record.Evidence
	for _, value := range []string{evidence.OwnerID, evidence.MediaApprovalRef, evidence.SourceApprovalRef, evidence.InterfaceVersion, evidence.RealCallEvidenceRef} {
		if !bounded(value, 128, false) {
			return deny("INVALID_EVIDENCE")
		}
	}
	if evidence.VerifiedAt.IsZero() || evidence.ExpiresAt.IsZero() || !evidence.ExpiresAt.After(evidence.VerifiedAt) || evidence.VerifiedAt.After(now) {
		return deny("INVALID_EVIDENCE")
	}
	if !now.Before(evidence.ExpiresAt) {
		return deny("VERIFICATION_EXPIRED")
	}
	return Decision{Allowed: true, Reason: "READY"}
}
