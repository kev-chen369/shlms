package promoter

import (
	"context"
	"errors"
	"testing"
)

type repoFunc func(context.Context, string) (Profile, error)

func (f repoFunc) FindByUserID(ctx context.Context, id string) (Profile, error) { return f(ctx, id) }

func TestGetProfile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record Profile
		err    error
		want   Status
		fails  bool
	}{
		{"absent", Profile{}, ErrNotFound, NotApplied, false},
		{"enabled", Profile{UserID: "u1", Status: Enabled}, nil, Enabled, false},
		{"other owner", Profile{UserID: "u2", Status: Enabled}, nil, "", true},
		{"corrupt status", Profile{UserID: "u1", Status: "BAD"}, nil, "", true},
		{"database error", Profile{}, errors.New("private diagnostic"), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Service{Repository: repoFunc(func(ctx context.Context, id string) (Profile, error) {
				if id != "u1" {
					t.Fatal("wrong owner")
				}
				return tc.record, tc.err
			})}
			p, err := s.GetProfile(context.Background(), "u1")
			if (err != nil) != tc.fails {
				t.Fatal(p, err)
			}
			if !tc.fails && (p.Status != tc.want || p.UserID != "u1") {
				t.Fatal(p)
			}
		})
	}
}

func TestGetProfileInvalidDependencyAndIdentity(t *testing.T) {
	if _, err := (Service{}).GetProfile(context.Background(), "u1"); err == nil {
		t.Fatal("nil repo accepted")
	}
	if _, err := (Service{}).GetProfile(context.Background(), " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
}
