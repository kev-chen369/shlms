package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/kev-chen369/shlms/internal/conversion"
)

func promotionShareArtifactHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(owner) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(q) != 1 || len(q["type"]) != 1 || (q.Get("type") != "link" && q.Get("type") != "text") {
			writeError(w, 400, "INVALID_REQUEST", "share type must be link or text")
			return
		}
		artifact, err := d.ShareArtifacts.GetArtifact(r.Context(), owner, r.PathValue("id"), q.Get("type"))
		if err != nil {
			switch {
			case errors.Is(err, conversion.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "share request is invalid")
			case errors.Is(err, conversion.ErrNotFound):
				writeError(w, 404, "CONVERSION_NOT_FOUND", "conversion request not found")
			case errors.Is(err, conversion.ErrShareNotReady):
				writeError(w, 409, "SHARE_NOT_READY", "share link is not ready")
			default:
				writeError(w, 503, "SHARE_UNAVAILABLE", "share content is unavailable")
			}
			return
		}
		if artifact.RequestID != r.PathValue("id") || artifact.Type != q.Get("type") {
			writeError(w, 503, "SHARE_UNAVAILABLE", "share content is unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": artifact})
	}
}
