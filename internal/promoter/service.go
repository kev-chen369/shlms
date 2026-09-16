package promoter

import (
	"context"
	"errors"
	"strings"
)

type Repository interface {
	FindByUserID(context.Context, string) (Profile, error)
}

type Service struct{ Repository Repository }

func (s Service) GetProfile(ctx context.Context, userID string) (Profile, error) {
	if strings.TrimSpace(userID) == "" {
		return Profile{}, ErrInvalidInput
	}
	if s.Repository == nil {
		return Profile{}, errors.New("promoter repository unavailable")
	}
	p, err := s.Repository.FindByUserID(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return Profile{UserID: userID, Status: NotApplied}, nil
	}
	if err != nil {
		return Profile{}, err
	}
	if p.UserID != userID {
		return Profile{}, errors.New("promoter repository owner mismatch")
	}
	switch p.Status {
	case NotApplied, Pending, Enabled, Rejected, Disabled:
		return p, nil
	default:
		return Profile{}, errors.New("invalid stored promoter status")
	}
}
