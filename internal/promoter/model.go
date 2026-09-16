// Package promoter owns promotion membership, independently of channel access.
package promoter

import (
	"errors"
	"strings"
	"time"
)

type Status string

const (
	NotApplied Status = "NOT_APPLIED"
	Pending    Status = "PENDING"
	Enabled    Status = "ENABLED"
	Rejected   Status = "REJECTED"
	Disabled   Status = "DISABLED"
)

var (
	ErrInvalidInput = errors.New("invalid promoter input")
	ErrTransition   = errors.New("invalid promoter transition")
	ErrConflict     = errors.New("promoter version conflict")
	ErrNotFound     = errors.New("promoter not found")
)

type Profile struct {
	UserID        string
	Status        Status
	Reason        string
	ApplicationID string
	Version       int64
}

type Capabilities struct {
	CanApply       bool `json:"canApply"`
	CanPromote     bool `json:"canPromote"`
	CanReadHistory bool `json:"canReadHistory"`
}

// Permissions only grant membership rights. Channel readiness, ownership and
// wallet risk controls must still be checked by the corresponding services.
func (p Profile) Permissions() Capabilities {
	switch p.Status {
	case NotApplied, Rejected:
		return Capabilities{CanApply: true}
	case Enabled:
		return Capabilities{CanPromote: true, CanReadHistory: true}
	case Disabled:
		return Capabilities{CanReadHistory: true}
	default:
		return Capabilities{}
	}
}

type Application struct {
	ID               string
	UserID           string
	DisplayName      string
	Scene            string
	AgreementVersion string
	ConsentedAt      time.Time
}

// Apply is a pure transition. A repository must persist the application and
// profile atomically and enforce idempotency and the expected version.
func (p Profile) Apply(a Application, expectedVersion int64) (Profile, error) {
	if p.Version != expectedVersion {
		return Profile{}, ErrConflict
	}
	if p.Status != NotApplied && p.Status != Rejected {
		return Profile{}, ErrTransition
	}
	if strings.TrimSpace(p.UserID) == "" || a.UserID != p.UserID ||
		strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.DisplayName) == "" ||
		strings.TrimSpace(a.Scene) == "" || strings.TrimSpace(a.AgreementVersion) == "" || a.ConsentedAt.IsZero() {
		return Profile{}, ErrInvalidInput
	}
	p.Status, p.ApplicationID, p.Reason = Pending, a.ID, ""
	p.Version++
	return p, nil
}

// Review and Disable are domain operations, not authorization checks. Only an
// authorized admin service may call them, persisting an audit in the same transaction.
func (p Profile) Review(approve bool, reason string, expectedVersion int64) (Profile, error) {
	if p.Version != expectedVersion {
		return Profile{}, ErrConflict
	}
	if p.Status != Pending {
		return Profile{}, ErrTransition
	}
	if strings.TrimSpace(reason) == "" {
		return Profile{}, ErrInvalidInput
	}
	p.Status = Rejected
	if approve {
		p.Status = Enabled
	}
	p.Reason = strings.TrimSpace(reason)
	p.Version++
	return p, nil
}

func (p Profile) Disable(reason string, expectedVersion int64) (Profile, error) {
	if p.Version != expectedVersion {
		return Profile{}, ErrConflict
	}
	if p.Status != Enabled {
		return Profile{}, ErrTransition
	}
	if strings.TrimSpace(reason) == "" {
		return Profile{}, ErrInvalidInput
	}
	p.Status, p.Reason = Disabled, strings.TrimSpace(reason)
	p.Version++
	return p, nil
}
