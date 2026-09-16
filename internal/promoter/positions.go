package promoter

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrNotEnabled = errors.New("promoter is not enabled")

const (
	PositionCreate  = "CREATE"
	PositionEdit    = "EDIT"
	PositionDefault = "DEFAULT"
	PositionDisable = "DISABLE"
)

type Position struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"-"`
	Name        string    `json:"name"`
	Scene       string    `json:"scene"`
	Status      Status    `json:"status"`
	IsDefault   bool      `json:"isDefault"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PositionCommand struct {
	Action         string
	ID             string
	Name           string
	Scene          string
	IsDefault      bool
	Version        int64
	IdempotencyKey string
}

type PositionListInput struct {
	Status Status
	Limit  int
	Cursor string
}
type PositionPage struct {
	Items      []Position `json:"items"`
	NextCursor string     `json:"nextCursor"`
}

type PositionRepository interface {
	ChangePosition(context.Context, string, PositionCommand) (Position, error)
	ListPositions(context.Context, string, PositionListInput) (PositionPage, error)
}
type PositionService struct{ Repository PositionRepository }

func (s PositionService) ChangePosition(ctx context.Context, userID string, c PositionCommand) (Position, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Scene = strings.TrimSpace(c.Scene)
	if !validText(userID, 256) || !validKey(c.IdempotencyKey) {
		return Position{}, ErrInvalidInput
	}
	switch c.Action {
	case PositionCreate:
		if c.ID != "" || c.Version != 0 || !validText(c.Name, 80) || !validText(c.Scene, 80) {
			return Position{}, ErrInvalidInput
		}
	case PositionEdit:
		if !validText(c.ID, 256) || c.Version < 1 || !validText(c.Name, 80) || !validText(c.Scene, 80) || c.IsDefault {
			return Position{}, ErrInvalidInput
		}
	case PositionDefault, PositionDisable:
		if !validText(c.ID, 256) || c.Version < 1 || c.Name != "" || c.Scene != "" || c.IsDefault {
			return Position{}, ErrInvalidInput
		}
	default:
		return Position{}, ErrInvalidInput
	}
	if s.Repository == nil {
		return Position{}, errors.New("position repository unavailable")
	}
	return s.Repository.ChangePosition(ctx, userID, c)
}

func (s PositionService) ListPositions(ctx context.Context, userID string, in PositionListInput) (PositionPage, error) {
	if !validText(userID, 256) {
		return PositionPage{}, ErrInvalidInput
	}
	if in.Limit == 0 {
		in.Limit = 20
	}
	if in.Limit < 1 || in.Limit > 100 || len(in.Cursor) > 2048 || (in.Status != "" && in.Status != Enabled && in.Status != Disabled) {
		return PositionPage{}, ErrInvalidInput
	}
	if s.Repository == nil {
		return PositionPage{}, errors.New("position repository unavailable")
	}
	return s.Repository.ListPositions(ctx, userID, in)
}
