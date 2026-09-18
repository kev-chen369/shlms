package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/conversion"
)

func promotionShareEventsHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
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
			EventID string `json:"eventId"`
			Action  string `json:"action"`
			Scene   string `json:"scene"`
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
		if key != body.EventID {
			writeError(w, 400, "INVALID_REQUEST", "Idempotency-Key must match eventId")
			return
		}
		result, err := d.NativeShareEvents.RecordShareEvent(r.Context(), conversion.NativeShareEventInput{OwnerUserID: userID, LinkID: r.PathValue("id"), EventID: body.EventID, Action: body.Action, Scene: body.Scene})
		if err != nil {
			switch {
			case errors.Is(err, conversion.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "share event is invalid")
			case errors.Is(err, conversion.ErrNotFound):
				writeError(w, 404, "CONVERSION_NOT_FOUND", "conversion request not found")
			case errors.Is(err, conversion.ErrStateConflict):
				writeError(w, 409, "LINK_NOT_READY", "a successful conversion is required")
			case errors.Is(err, conversion.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "eventId belongs to another operation")
			default:
				writeError(w, 503, "SHARE_UNAVAILABLE", "share event service is unavailable")
			}
			return
		}
		if result.EventID != body.EventID || result.LinkID != r.PathValue("id") || result.Action != body.Action || result.Scene != body.Scene || result.RecordedAt.IsZero() {
			writeError(w, 503, "SHARE_UNAVAILABLE", "share event service is unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "recorded", "data": map[string]any{
			"eventId": result.EventID, "linkId": result.LinkID, "trackingId": result.TrackingID, "action": result.Action, "scene": result.Scene, "recordedAt": result.RecordedAt,
		}})
	}
}
