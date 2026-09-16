package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/kev-chen369/shlms/internal/promoter"
)

// AdminResolver supplies an authenticated actor and server-side permissions.
// Ordinary user authentication alone must never satisfy this dependency.
type AdminResolver interface {
	ResolveAdmin(*http.Request) (promoter.AdminActor, error)
}
type PromoterAdministrator interface {
	Execute(context.Context, promoter.AdminActor, promoter.AdminCommand) (promoter.Profile, error)
}

func promoterAdminHandler(d Dependencies, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor, ok := resolveAdmin(w, r, d.Admins)
		if !ok {
			return
		}
		permission := promoter.ReviewPermission
		if action == promoter.DisableAction {
			permission = promoter.DisablePermission
		}
		if !actor.Permissions[permission] {
			writeError(w, 403, "FORBIDDEN", "administrator permission required")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			writeError(w, 415, "UNSUPPORTED_MEDIA_TYPE", "application/json is required")
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			writeError(w, 400, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
			return
		}
		var body struct {
			Approve *bool  `json:"approve"`
			Reason  string `json:"reason"`
			Version *int64 `json:"version"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&body)
		if err == nil {
			var extra any
			err = decoder.Decode(&extra)
			if errors.Is(err, io.EOF) {
				err = nil
			} else if err == nil {
				err = errors.New("multiple JSON values")
			}
		}
		if err != nil {
			var large *http.MaxBytesError
			if errors.As(err, &large) {
				writeError(w, 413, "REQUEST_TOO_LARGE", "request exceeds size limit")
			} else {
				writeError(w, 400, "INVALID_REQUEST", "request body is invalid")
			}
			return
		}
		if body.Version == nil || (action == promoter.ReviewAction && body.Approve == nil) || (action == promoter.DisableAction && body.Approve != nil) {
			writeError(w, 400, "INVALID_REQUEST", "version and explicit review decision required; disable does not accept approve")
			return
		}
		c := promoter.AdminCommand{Action: action, TargetID: r.PathValue("id"), Reason: body.Reason, Version: *body.Version, IdempotencyKey: key}
		if body.Approve != nil {
			c.Approve = *body.Approve
		}
		p, err := d.PromoterAdmin.Execute(r.Context(), actor, c)
		if err != nil {
			switch {
			case errors.Is(err, promoter.ErrForbidden):
				writeError(w, 403, "FORBIDDEN", "operation is not permitted")
			case errors.Is(err, promoter.ErrNotFound):
				writeError(w, 404, "PROMOTER_NOT_FOUND", "application or promoter not found")
			case errors.Is(err, promoter.ErrInvalidInput):
				writeError(w, 400, "INVALID_REQUEST", "admin request is invalid")
			case errors.Is(err, promoter.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to a different request")
			case errors.Is(err, promoter.ErrConflict):
				writeError(w, 409, "VERSION_CONFLICT", "refresh the current application before retrying")
			case errors.Is(err, promoter.ErrTransition):
				writeError(w, 409, "INVALID_TRANSITION", "current status does not allow this action")
			default:
				writeError(w, 503, "PROMOTER_UNAVAILABLE", "admin service is unavailable")
			}
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{"userId": p.UserID, "applicationId": p.ApplicationID, "status": p.Status, "version": p.Version}})
	}
}
