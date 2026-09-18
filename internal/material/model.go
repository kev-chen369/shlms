// Package material models trusted catalog metadata, not prices or permission to
// generate links. Imports and network URLs still require channel verification.
package material

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid promotion material")
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Region struct {
	Mode      string   `json:"mode"`
	CityCodes []string `json:"cityCodes,omitempty"`
}

// Record is internal metadata. Never serialize it as a public API response.
type Record struct {
	ID, Platform, Type, ExternalMaterialID string
	CanonicalURL                           string `json:"-"`
	Title, Status                          string
	StartsAt, EndsAt, SourceUpdatedAt      time.Time
	RuleVersion                            string
	EvidenceRef                            string `json:"-"`
	Region                                 Region
	Business                               string
	Terminals                              []string
}
type Context struct{ Platform, Type, CityCode, Business, Terminal string }
type Decision struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// Card deliberately has no URL, external ID, audit reference, price, estimate,
// or canGenerate flag. Availability must not bypass the capability policy.
type Card struct {
	ID              string     `json:"id"`
	Platform        string     `json:"platform"`
	Type            string     `json:"type"`
	Title           string     `json:"title"`
	StartsAt        *time.Time `json:"startsAt,omitempty"`
	EndsAt          time.Time  `json:"endsAt"`
	SourceUpdatedAt time.Time  `json:"sourceUpdatedAt"`
	RuleVersion     string     `json:"ruleVersion"`
	Region          Region     `json:"region"`
	Business        string     `json:"business"`
	Terminals       []string   `json:"terminals"`
}

func text(value string, limit int, optional bool) bool {
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
func platformType(platform, kind string) bool {
	return (platform == "JD" || platform == "TB" || platform == "MT") && (kind == "PRODUCT" || kind == "ACTIVITY") && (platform != "MT" || kind == "ACTIVITY")
}
func terminal(value string) bool { return value == "H5" || value == "WX_MINI" }
func validRegion(region Region) bool {
	switch region.Mode {
	case "NATIONWIDE":
		return len(region.CityCodes) == 0
	case "CITIES":
		if len(region.CityCodes) == 0 || len(region.CityCodes) > 64 {
			return false
		}
		seen := map[string]bool{}
		for _, code := range region.CityCodes {
			if !text(code, 32, false) || seen[code] {
				return false
			}
			seen[code] = true
		}
		return true
	default:
		return false
	}
}

// Syntax only: DNS, redirects, allowed hosts, media and channel authorization
// must be checked independently before using this internal URL over a network.
func validURL(value string) bool {
	if !text(value, 2048, false) || strings.ContainsRune(value, '\\') || strings.ContainsFunc(value, unicode.IsSpace) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil && parsed.Opaque == ""
}

func validTime(value time.Time, optional bool) bool {
	if value.IsZero() {
		return optional
	}
	_, err := value.MarshalJSON()
	return err == nil
}

func Validate(record Record) error {
	if !uuid.MatchString(record.ID) || record.ID == "00000000-0000-0000-0000-000000000000" || !platformType(record.Platform, record.Type) || !text(record.ExternalMaterialID, 128, false) || !validURL(record.CanonicalURL) || !text(record.Title, 256, false) || !text(record.RuleVersion, 80, false) || !text(record.EvidenceRef, 128, false) || !text(record.Business, 40, true) || !validRegion(record.Region) {
		return ErrInvalid
	}
	if !slices.Contains([]string{"DRAFT", "ACTIVE", "SUSPENDED", "REMOVED"}, record.Status) || len(record.Terminals) == 0 || len(record.Terminals) > 2 {
		return ErrInvalid
	}
	for i, value := range record.Terminals {
		if !terminal(value) || (i > 0 && value == record.Terminals[0]) {
			return ErrInvalid
		}
	}
	if !validTime(record.EndsAt, false) || !validTime(record.SourceUpdatedAt, false) || !validTime(record.StartsAt, record.Type == "PRODUCT") || (!record.StartsAt.IsZero() && !record.EndsAt.After(record.StartsAt)) {
		return ErrInvalid
	}
	return nil
}

// CardFor returns zero data on any denial. now comes from the server; record
// from trusted imports. This never grants identity, position or link readiness.
func CardFor(record Record, context Context, now time.Time) (Card, Decision) {
	deny := func(reason string) (Card, Decision) { return Card{}, Decision{Reason: reason} }
	if Validate(record) != nil {
		return deny("INVALID_MATERIAL")
	}
	if now.IsZero() || !platformType(context.Platform, context.Type) || !terminal(context.Terminal) || !text(context.CityCode, 32, true) || !text(context.Business, 40, true) {
		return deny("INVALID_CONTEXT")
	}
	if context.Platform != record.Platform || context.Type != record.Type {
		return deny("SCOPE_MISMATCH")
	}
	if record.Status != "ACTIVE" {
		return deny("NOT_ACTIVE")
	}
	if record.StartsAt.After(now) {
		return deny("NOT_STARTED")
	}
	if !now.Before(record.EndsAt) {
		return deny("EXPIRED")
	}
	if record.SourceUpdatedAt.After(now) {
		return deny("SOURCE_NOT_CURRENT")
	}
	if record.Region.Mode == "CITIES" && !slices.Contains(record.Region.CityCodes, context.CityCode) {
		return deny("REGION_MISMATCH")
	}
	if record.Business != "" && record.Business != context.Business {
		return deny("BUSINESS_MISMATCH")
	}
	if !slices.Contains(record.Terminals, context.Terminal) {
		return deny("TERMINAL_MISMATCH")
	}
	var startsAt *time.Time
	if !record.StartsAt.IsZero() {
		value := record.StartsAt
		startsAt = &value
	}
	return Card{ID: record.ID, Platform: record.Platform, Type: record.Type, Title: record.Title, StartsAt: startsAt, EndsAt: record.EndsAt, SourceUpdatedAt: record.SourceUpdatedAt, RuleVersion: record.RuleVersion, Region: Region{Mode: record.Region.Mode, CityCodes: slices.Clone(record.Region.CityCodes)}, Business: record.Business, Terminals: slices.Clone(record.Terminals)}, Decision{Available: true, Reason: "AVAILABLE"}
}
