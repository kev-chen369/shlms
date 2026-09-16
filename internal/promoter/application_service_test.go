package promoter

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type writerFunc func(context.Context, string, Application) (Application, error)

func (f writerFunc) Submit(ctx context.Context, key string, a Application) (Application, error) {
	return f(ctx, key, a)
}

func submitInput() SubmitInput {
	return SubmitInput{UserID: "u1", IdempotencyKey: "key", DisplayName: " 姓名 ", Scene: " 群分享 ", AgreementVersion: "v1", Agreed: true}
}

func TestApplicationServiceNormalizesAndRecordsConsent(t *testing.T) {
	now := time.Date(2026, 9, 16, 0, 0, 0, 123456789, time.UTC)
	s := ApplicationService{AgreementVersion: "v1", Now: func() time.Time { return now }, Repository: writerFunc(func(ctx context.Context, key string, a Application) (Application, error) {
		if key != "key" || a.DisplayName != "姓名" || a.Scene != "群分享" || a.UserID != "u1" || !a.ConsentedAt.Equal(now.Truncate(time.Microsecond)) || !strings.HasPrefix(a.ID, "PA-") {
			t.Fatal(key, a)
		}
		return a, nil
	})}
	a, err := s.SubmitApplication(context.Background(), submitInput())
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.SubmitApplication(context.Background(), submitInput())
	if err != nil || a.ID == b.ID {
		t.Fatal("IDs must be unique", err)
	}
}

func TestApplicationServiceValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*SubmitInput)
		want   error
	}{
		{"no consent", func(in *SubmitInput) { in.Agreed = false }, ErrAgreement},
		{"old agreement", func(in *SubmitInput) { in.AgreementVersion = "old" }, ErrAgreement},
		{"no user", func(in *SubmitInput) { in.UserID = "" }, ErrInvalidInput},
		{"long name", func(in *SubmitInput) { in.DisplayName = strings.Repeat("中", 81) }, ErrInvalidInput},
		{"control", func(in *SubmitInput) { in.Scene = "群\x00分享" }, ErrInvalidInput},
		{"empty scene", func(in *SubmitInput) { in.Scene = " " }, ErrInvalidInput},
		{"invalid key", func(in *SubmitInput) { in.IdempotencyKey = "bad key" }, ErrInvalidInput},
		{"long key", func(in *SubmitInput) { in.IdempotencyKey = strings.Repeat("a", 129) }, ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ApplicationService{AgreementVersion: "v1", Repository: writerFunc(func(context.Context, string, Application) (Application, error) {
				t.Fatal("invalid submission reached repository")
				return Application{}, nil
			})}
			in := submitInput()
			tc.mutate(&in)
			if _, err := s.SubmitApplication(context.Background(), in); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	if _, err := (ApplicationService{}).SubmitApplication(context.Background(), submitInput()); err == nil {
		t.Fatal("unconfigured service accepted request")
	}
}
