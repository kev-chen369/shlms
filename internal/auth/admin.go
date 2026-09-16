package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/kev-chen369/shlms/internal/promoter"
)

var ErrUnavailable = errors.New("administrator authorization unavailable")

type AdminPermissionStore interface {
	PermissionsForUser(context.Context, string) (map[string]bool, error)
}

type AdminResolver struct {
	Verifier Verifier
	Store    AdminPermissionStore
}

func (r AdminResolver) ResolveAdmin(request *http.Request) (promoter.AdminActor, error) {
	userID, err := r.Verifier.ResolveUserID(request)
	if err != nil {
		return promoter.AdminActor{}, ErrUnauthorized
	}
	if r.Store == nil {
		return promoter.AdminActor{}, ErrUnavailable
	}
	permissions, err := r.Store.PermissionsForUser(request.Context(), userID)
	if err != nil {
		return promoter.AdminActor{}, ErrUnavailable
	}
	if permissions == nil {
		permissions = map[string]bool{}
	}
	return promoter.AdminActor{ID: userID, Permissions: permissions}, nil
}

type PostgresAdminStore struct{ DB *sql.DB }

func (s PostgresAdminStore) PermissionsForUser(ctx context.Context, userID string) (map[string]bool, error) {
	if s.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT perm.permission FROM admin_principals principal
		JOIN admin_permissions perm ON perm.user_id=principal.user_id
		WHERE principal.user_id=$1 AND principal.active=true`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	permissions := map[string]bool{}
	for rows.Next() {
		var permission string
		if err = rows.Scan(&permission); err != nil {
			return nil, err
		}
		permissions[permission] = true
	}
	return permissions, rows.Err()
}
