package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type PromoterAdminLister interface {
	ListApplications(context.Context, promoter.AdminActor, promoter.ApplicationListInput) (promoter.ApplicationPage, error)
}

func promoterAdminListHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor, err := d.Admins.ResolveAdmin(r)
		if err != nil || strings.TrimSpace(actor.ID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "administrator authentication required")
			return
		}
		if !actor.Permissions[promoter.ReadPermission] {
			writeError(w, 403, "FORBIDDEN", "application read permission required")
			return
		}
		query := r.URL.Query()
		for key, values := range query {
			if (key != "status" && key != "limit" && key != "cursor") || len(values) != 1 {
				writeError(w, 400, "INVALID_REQUEST", "unsupported or repeated query parameter")
				return
			}
		}
		limit := 20
		if values, ok := query["limit"]; ok {
			limit, err = strconv.Atoi(values[0])
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, 400, "INVALID_REQUEST", "limit must be between 1 and 100")
				return
			}
		}
		page, err := d.PromoterAdminList.ListApplications(r.Context(), actor, promoter.ApplicationListInput{Status: promoter.Status(query.Get("status")), Limit: limit, Cursor: query.Get("cursor")})
		if err != nil {
			switch {
			case errors.Is(err, promoter.ErrForbidden):
				writeError(w, 403, "FORBIDDEN", "application read permission required")
			case errors.Is(err, promoter.ErrInvalidInput):
				writeError(w, 400, "INVALID_REQUEST", "invalid filter or cursor")
			default:
				writeError(w, 503, "PROMOTER_UNAVAILABLE", "application list is unavailable")
			}
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": page})
	}
}
