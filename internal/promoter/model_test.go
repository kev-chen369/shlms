package promoter

import (
	"errors"
	"testing"
	"time"
)

func application() Application {
	return Application{ID: "app-1", UserID: "u1", DisplayName: "推广员", Scene: "群分享", AgreementVersion: "v1", ConsentedAt: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)}
}

func TestMembershipLifecycle(t *testing.T) {
	initial := Profile{UserID: "u1", Status: NotApplied}
	pending, err := initial.Apply(application(), 0)
	if err != nil || pending.Status != Pending || pending.Version != 1 || pending.ApplicationID != "app-1" {
		t.Fatalf("apply = %+v, %v", pending, err)
	}
	if initial.Status != NotApplied {
		t.Fatal("transition mutated original")
	}
	approved, err := pending.Review(true, "审核通过", 1)
	if err != nil || approved.Status != Enabled || approved.Version != 2 {
		t.Fatalf("review = %+v, %v", approved, err)
	}
	stopped, err := approved.Disable("违规停用", 2)
	if err != nil || stopped.Status != Disabled || stopped.Version != 3 || stopped.ApplicationID != "app-1" {
		t.Fatalf("disable = %+v, %v", stopped, err)
	}
	if stopped.Permissions().CanPromote || !stopped.Permissions().CanReadHistory {
		t.Fatal("disabled permissions unsafe")
	}
	if _, err := stopped.Apply(application(), 3); !errors.Is(err, ErrTransition) {
		t.Fatalf("disabled reapply: %v", err)
	}
}

func TestRejectedMayReapply(t *testing.T) {
	p := Profile{UserID: "u1", Status: Pending, ApplicationID: "old", Version: 1}
	p, err := p.Review(false, "资料不完整", 1)
	if err != nil || p.Status != Rejected {
		t.Fatal(p, err)
	}
	p, err = p.Apply(application(), 2)
	if err != nil || p.Status != Pending || p.Reason != "" || p.ApplicationID != "app-1" {
		t.Fatal(p, err)
	}
}

func TestApplyRequiresValidConsentAndOwner(t *testing.T) {
	cases := map[string]func(*Application){
		"owner":     func(a *Application) { a.UserID = "other" },
		"id":        func(a *Application) { a.ID = " " },
		"name":      func(a *Application) { a.DisplayName = " " },
		"scene":     func(a *Application) { a.Scene = "" },
		"agreement": func(a *Application) { a.AgreementVersion = "" },
		"consent":   func(a *Application) { a.ConsentedAt = time.Time{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := application()
			mutate(&a)
			if _, err := (Profile{UserID: "u1", Status: NotApplied}).Apply(a, 0); !errors.Is(err, ErrInvalidInput) {
				t.Fatal(err)
			}
		})
	}
}

func TestStateAndVersionGuards(t *testing.T) {
	for _, status := range []Status{NotApplied, Pending, Enabled, Rejected, Disabled, "UNKNOWN"} {
		p := Profile{UserID: "u1", Status: status, Version: 5}
		if _, err := p.Apply(application(), 4); !errors.Is(err, ErrConflict) {
			t.Fatal(status, err)
		}
		if _, err := p.Review(true, "ok", 4); !errors.Is(err, ErrConflict) {
			t.Fatal(status, err)
		}
		if _, err := p.Disable("reason", 4); !errors.Is(err, ErrConflict) {
			t.Fatal(status, err)
		}
		if status != NotApplied && status != Rejected {
			if _, err := p.Apply(application(), 5); !errors.Is(err, ErrTransition) {
				t.Fatal(status, err)
			}
		}
		if status != Pending {
			if _, err := p.Review(true, "ok", 5); !errors.Is(err, ErrTransition) {
				t.Fatal(status, err)
			}
		}
		if status != Enabled {
			if _, err := p.Disable("reason", 5); !errors.Is(err, ErrTransition) {
				t.Fatal(status, err)
			}
		}
	}
	if _, err := (Profile{Status: Pending}).Review(false, " ", 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err := (Profile{Status: Enabled}).Disable(" ", 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
}

func TestPermissions(t *testing.T) {
	for _, tc := range []struct {
		status Status
		want   Capabilities
	}{
		{NotApplied, Capabilities{CanApply: true}}, {Rejected, Capabilities{CanApply: true}},
		{Pending, Capabilities{}}, {Enabled, Capabilities{CanPromote: true, CanReadHistory: true}},
		{Disabled, Capabilities{CanReadHistory: true}}, {"UNKNOWN", Capabilities{}},
	} {
		if got := (Profile{Status: tc.status}).Permissions(); got != tc.want {
			t.Fatalf("%s: %+v", tc.status, got)
		}
	}
}
