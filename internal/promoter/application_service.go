package promoter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrAgreement = errors.New("current agreement acceptance required")

type ApplicationWriter interface {
	Submit(context.Context, string, Application) (Application, error)
}

type ApplicationService struct {
	Repository       ApplicationWriter
	AgreementVersion string
	Now              func() time.Time
}

type SubmitInput struct {
	UserID           string
	IdempotencyKey   string
	DisplayName      string
	Scene            string
	AgreementVersion string
	Agreed           bool
}

func (s ApplicationService) SubmitApplication(ctx context.Context, in SubmitInput) (Application, error) {
	if s.Repository == nil || strings.TrimSpace(s.AgreementVersion) == "" {
		return Application{}, errors.New("application service unconfigured")
	}
	if !in.Agreed || in.AgreementVersion != s.AgreementVersion {
		return Application{}, ErrAgreement
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Scene = strings.TrimSpace(in.Scene)
	if !validText(in.UserID, 256) || !validText(in.DisplayName, 80) || !validText(in.Scene, 80) || !validText(in.AgreementVersion, 80) || !validKey(in.IdempotencyKey) {
		return Application{}, ErrInvalidInput
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Application{}, err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	a := Application{ID: "PA-" + hex.EncodeToString(id[:]), UserID: in.UserID, DisplayName: in.DisplayName, Scene: in.Scene, AgreementVersion: in.AgreementVersion, ConsentedAt: now().UTC().Truncate(time.Microsecond)}
	return s.Repository.Submit(ctx, in.IdempotencyKey, a)
}

func validText(s string, max int) bool {
	if !utf8.ValidString(s) || strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validKey(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}
