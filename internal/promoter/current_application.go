package promoter

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type CurrentApplication struct {
	Application Application
	Status      Status
	Reason      string
}

type CurrentApplicationRepository interface {
	FindCurrentApplication(context.Context, string) (CurrentApplication, error)
}

type CurrentApplicationService struct{ Repository CurrentApplicationRepository }

func (s CurrentApplicationService) GetCurrentApplication(ctx context.Context, userID string) (CurrentApplication, error) {
	if strings.TrimSpace(userID) == "" {
		return CurrentApplication{}, ErrInvalidInput
	}
	if s.Repository == nil {
		return CurrentApplication{}, errors.New("application reader unconfigured")
	}
	a, err := s.Repository.FindCurrentApplication(ctx, userID)
	if err != nil {
		return CurrentApplication{}, err
	}
	if a.Application.UserID != userID || a.Application.ID == "" {
		return CurrentApplication{}, errors.New("invalid current application owner")
	}
	switch a.Status {
	case Pending, Enabled, Rejected, Disabled:
	default:
		return CurrentApplication{}, errors.New("invalid application state")
	}
	if a.Status != Rejected && a.Status != Disabled {
		a.Reason = ""
	}
	return a, nil
}

func (r PostgresRepository) FindCurrentApplication(ctx context.Context, userID string) (CurrentApplication, error) {
	var current CurrentApplication
	a := &current.Application
	// One snapshot: never combine an old application with a concurrently updated profile.
	err := r.db.QueryRowContext(ctx, `SELECT a.id,a.user_id,a.display_name,a.scene,a.agreement_version,a.consented_at,p.status,p.reason
		FROM promoter_profiles p JOIN promoter_applications a ON a.id=p.application_id AND a.user_id=p.user_id WHERE p.user_id=$1`, userID).Scan(&a.ID, &a.UserID, &a.DisplayName, &a.Scene, &a.AgreementVersion, &a.ConsentedAt, &current.Status, &current.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return CurrentApplication{}, ErrNotFound
	}
	return current, err
}
