package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type ChannelPositionConfigurator interface {
	Configure(context.Context, promoter.AdminActor, promoter.ConfigureChannelPositionInput) (promoter.ChannelPosition, error)
}

func channelPositionConfigHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor, err := d.Admins.ResolveAdmin(r)
		if err != nil || strings.TrimSpace(actor.ID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "administrator authentication required")
			return
		}
		if !actor.Permissions[promoter.ConfigureChannelPermission] {
			writeError(w, 403, "FORBIDDEN", "channel position permission required")
			return
		}
		if r.PathValue("channel") != "JD" {
			writeError(w, 422, "UNSUPPORTED_CHANNEL", "channel mapping is unsupported")
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
			AccountID          *string `json:"accountId"`
			ExternalPositionID *string `json:"externalPositionId"`
			Version            *int64  `json:"version"`
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
		if body.AccountID == nil || body.ExternalPositionID == nil || body.Version == nil {
			writeError(w, 400, "INVALID_REQUEST", "accountId, externalPositionId and version are required")
			return
		}
		in := promoter.ConfigureChannelPositionInput{PositionID: r.PathValue("id"), Channel: "JD", AccountID: *body.AccountID, ExternalPositionID: *body.ExternalPositionID, ExpectedVersion: *body.Version, IdempotencyKey: key}
		mapping, err := d.ChannelPositions.Configure(r.Context(), actor, in)
		if err != nil {
			switch {
			case errors.Is(err, promoter.ErrForbidden):
				writeError(w, 403, "FORBIDDEN", "channel position permission required")
			case errors.Is(err, promoter.ErrInvalidInput):
				writeError(w, 400, "INVALID_REQUEST", "channel position request is invalid")
			case errors.Is(err, promoter.ErrNotFound):
				writeError(w, 404, "POSITION_NOT_FOUND", "position not found")
			case errors.Is(err, promoter.ErrNotEnabled):
				writeError(w, 403, "PROMOTER_NOT_ENABLED", "active promoter membership required")
			case errors.Is(err, promoter.ErrTransition):
				writeError(w, 409, "POSITION_DISABLED", "disabled positions cannot be configured")
			case errors.Is(err, promoter.ErrConflict):
				writeError(w, 409, "VERSION_CONFLICT", "refresh channel position configuration")
			case errors.Is(err, promoter.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to a different configuration")
			case errors.Is(err, promoter.ErrExternalPositionConflict):
				writeError(w, 409, "EXTERNAL_POSITION_CONFLICT", "external position is already assigned")
			default:
				writeError(w, 503, "CHANNEL_POSITION_UNAVAILABLE", "channel position service is unavailable")
			}
			return
		}
		if mapping.PositionID != in.PositionID || mapping.Channel != "JD" || mapping.Status != "PENDING_VERIFICATION" {
			writeError(w, 503, "CHANNEL_POSITION_UNAVAILABLE", "channel position service is unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": mapping})
	}
}
