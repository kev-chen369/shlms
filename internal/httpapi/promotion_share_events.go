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

func promotionShareEventHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(owner) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			writeError(w, 415, "UNSUPPORTED_MEDIA_TYPE", "application/json is required")
			return
		}
		var body struct {
			EventID      string `json:"eventId"`
			ArtifactType string `json:"artifactType"`
			Scene        string `json:"scene"`
			Action       string `json:"action"`
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
		event, err := d.ShareEvents.Record(r.Context(), conversion.ShareEventInput{OwnerUserID: owner, RequestID: r.PathValue("id"), EventID: body.EventID, ArtifactType: body.ArtifactType, Scene: body.Scene, Action: body.Action})
		if err != nil {
			switch {
			case errors.Is(err, conversion.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "share event is invalid")
			case errors.Is(err, conversion.ErrNotFound):
				writeError(w, 404, "CONVERSION_NOT_FOUND", "conversion request not found")
			case errors.Is(err, conversion.ErrShareNotReady):
				writeError(w, 409, "SHARE_NOT_READY", "share link is not ready")
			case errors.Is(err, conversion.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "event ID belongs to another report")
			default:
				writeError(w, 503, "SHARE_UNAVAILABLE", "share event is unavailable")
			}
			return
		}
		if event.RequestID != r.PathValue("id") || event.EventID != body.EventID || event.Action != "COPY_REPORTED" {
			writeError(w, 503, "SHARE_UNAVAILABLE", "share event is unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "recorded", "data": event})
	}
}
