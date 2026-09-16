package promoter

import (
	"context"
	"errors"
	"testing"
)

type currentRepoFunc func(context.Context, string) (CurrentApplication, error)

func (f currentRepoFunc) FindCurrentApplication(ctx context.Context, id string) (CurrentApplication, error) {
	return f(ctx, id)
}

func TestCurrentApplicationService(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  Status
		owner   string
		repoErr error
		fail    bool
		reason  string
	}{
		{"pending", Pending, "u1", nil, false, ""}, {"approved", Enabled, "u1", nil, false, ""},
		{"rejected", Rejected, "u1", nil, false, "reason"}, {"disabled", Disabled, "u1", nil, false, "reason"},
		{"wrong owner", Pending, "other", nil, true, ""}, {"unknown state", "bad", "u1", nil, true, ""},
		{"missing", Pending, "u1", ErrNotFound, true, ""}, {"db failed", Pending, "u1", errors.New("private"), true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := CurrentApplicationService{Repository: currentRepoFunc(func(ctx context.Context, id string) (CurrentApplication, error) {
				if id != "u1" {
					t.Fatal(id)
				}
				a := application()
				a.UserID = tc.owner
				return CurrentApplication{Application: a, Status: tc.status, Reason: "reason"}, tc.repoErr
			})}
			got, err := s.GetCurrentApplication(context.Background(), "u1")
			if (err != nil) != tc.fail {
				t.Fatal(got, err)
			}
			if !tc.fail && got.Reason != tc.reason {
				t.Fatal(got)
			}
		})
	}
	if _, err := (CurrentApplicationService{}).GetCurrentApplication(context.Background(), "u1"); err == nil {
		t.Fatal("nil repository")
	}
	if _, err := (CurrentApplicationService{}).GetCurrentApplication(context.Background(), " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
}

func TestPostgresCurrentApplication(t *testing.T) {
	db := promoterDB(t)
	repo := NewPostgresRepository(db)
	ctx := context.Background()
	if _, err := repo.FindCurrentApplication(ctx, "u1"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	a := application()
	if _, err := repo.Submit(ctx, "key", a); err != nil {
		t.Fatal(err)
	}
	got, err := (CurrentApplicationService{Repository: repo}).GetCurrentApplication(ctx, "u1")
	if err != nil || got.Application.ID != a.ID || got.Status != Pending || got.Application.AgreementVersion != a.AgreementVersion {
		t.Fatal(got, err)
	}
	if _, err := repo.FindCurrentApplication(ctx, "u2"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
