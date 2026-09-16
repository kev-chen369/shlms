package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/kev-chen369/shlms/internal/conversion"
	"github.com/kev-chen369/shlms/internal/linkresolve"
)

// Artifacts are read-only views of an existing successful conversion. Reading
// them does not create tracking, copy to a clipboard, or record delivery.
func promotionShareArtifactsHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(query) != 1 || len(query["type"]) != 1 {
			writeError(w, 400, "INVALID_REQUEST", "exactly one artifact type is required")
			return
		}
		kind := query.Get("type")
		if kind == "qr" || kind == "poster" {
			writeError(w, 422, "ARTIFACT_NOT_AVAILABLE", "requested material is not available")
			return
		}
		if kind != "link" && kind != "text" {
			writeError(w, 400, "INVALID_REQUEST", "artifact type must be link or text")
			return
		}
		record, err := d.ConversionReader.Get(r.Context(), userID, r.PathValue("id"))
		if err != nil {
			switch {
			case errors.Is(err, conversion.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "conversion request ID is invalid")
			case errors.Is(err, conversion.ErrNotFound):
				writeError(w, 404, "CONVERSION_NOT_FOUND", "conversion request not found")
			default:
				writeError(w, 503, "SHARE_UNAVAILABLE", "share content is unavailable")
			}
			return
		}
		if record.OwnerUserID != userID || record.ID != r.PathValue("id") {
			writeError(w, 404, "CONVERSION_NOT_FOUND", "conversion request not found")
			return
		}
		if record.Status != "SUCCEEDED" {
			writeError(w, 409, "LINK_NOT_READY", "a successful conversion is required")
			return
		}
		// The approved Gateway validated the host before persisting success.
		// This is only a format check on stored data, not channel approval or a fetch.
		u, err := url.Parse(record.LinkURL)
		if err != nil || u == nil {
			writeError(w, 503, "SHARE_UNAVAILABLE", "share content is unavailable")
			return
		}
		policy, err := linkresolve.NewPolicy([]string{u.Hostname()})
		if err == nil {
			_, err = policy.Validate(record.LinkURL)
		}
		if err != nil {
			writeError(w, 503, "SHARE_UNAVAILABLE", "share content is unavailable")
			return
		}
		content := record.LinkURL
		if kind == "text" {
			content = "万惠宝好物推荐\n" + record.LinkURL
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]string{
			"linkId": record.ID, "trackingId": record.TrackingID, "type": kind,
			"linkUrl": record.LinkURL, "content": content, "templateVersion": "v1",
		}})
	}
}
