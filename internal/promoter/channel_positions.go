package promoter

import (
	"context"
	"errors"
	"strings"
	"time"
)

const ConfigureChannelPermission = "channel:position:configure"

var ErrExternalPositionConflict = errors.New("external position already assigned")

type ChannelPosition struct {
	PositionID         string    `json:"positionId"`
	Channel            string    `json:"channel"`
	AccountID          string    `json:"accountId"`
	ExternalPositionID string    `json:"externalPositionId"`
	Status             string    `json:"status"`
	Version            int64     `json:"version"`
	ConfiguredAt       time.Time `json:"configuredAt"`
}

type ConfigureChannelPositionInput struct {
	PositionID         string
	Channel            string
	AccountID          string
	ExternalPositionID string
	ExpectedVersion    int64
	IdempotencyKey     string
}

type ChannelPositionRepository interface {
	ConfigureChannelPosition(context.Context, string, ConfigureChannelPositionInput) (ChannelPosition, error)
}
type ChannelPositionService struct{ Repository ChannelPositionRepository }

func (s ChannelPositionService) Configure(ctx context.Context, actor AdminActor, in ConfigureChannelPositionInput) (ChannelPosition, error) {
	if strings.TrimSpace(actor.ID) == "" || !actor.Permissions[ConfigureChannelPermission] {
		return ChannelPosition{}, ErrForbidden
	}
	in.AccountID = strings.TrimSpace(in.AccountID)
	in.ExternalPositionID = strings.TrimSpace(in.ExternalPositionID)
	if in.Channel != "JD" || !validText(in.PositionID, 256) || !validText(in.AccountID, 128) || !validText(in.ExternalPositionID, 128) || !validKey(in.IdempotencyKey) || in.ExpectedVersion < 0 {
		return ChannelPosition{}, ErrInvalidInput
	}
	if s.Repository == nil {
		return ChannelPosition{}, errors.New("channel position repository unavailable")
	}
	return s.Repository.ConfigureChannelPosition(ctx, actor.ID, in)
}
