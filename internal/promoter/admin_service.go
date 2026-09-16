package promoter

import (
	"context"
	"errors"
	"strings"
)

var ErrForbidden = errors.New("promoter admin permission required")

const (
	ReviewAction      = "REVIEW"
	DisableAction     = "DISABLE"
	ReviewPermission  = "promoter:review"
	DisablePermission = "promoter:disable"
)

// Actor must come from a trusted administrator identity provider, never JSON.
type AdminActor struct {
	ID          string
	Permissions map[string]bool
}

type AdminCommand struct {
	Action         string
	TargetID       string
	IdempotencyKey string
	Approve        bool
	Reason         string // user-visible reason; do not include confidential reviewer notes.
	Version        int64
}

type AdminWriter interface {
	ExecuteAdmin(context.Context, string, AdminCommand) (Profile, error)
}
type AdminService struct{ Repository AdminWriter }

func (s AdminService) Execute(ctx context.Context, actor AdminActor, command AdminCommand) (Profile, error) {
	permission := ReviewPermission
	if command.Action == DisableAction {
		permission = DisablePermission
	} else if command.Action != ReviewAction {
		return Profile{}, ErrInvalidInput
	}
	if strings.TrimSpace(actor.ID) == "" || !actor.Permissions[permission] {
		return Profile{}, ErrForbidden
	}
	command.Reason = strings.TrimSpace(command.Reason)
	if !validText(actor.ID, 256) || !validText(command.TargetID, 256) || !validText(command.Reason, 500) || !validKey(command.IdempotencyKey) || command.Version < 1 || (command.Action == DisableAction && command.Approve) {
		return Profile{}, ErrInvalidInput
	}
	if s.Repository == nil {
		return Profile{}, errors.New("admin repository unavailable")
	}
	return s.Repository.ExecuteAdmin(ctx, actor.ID, command)
}
