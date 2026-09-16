package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/kev-chen369/shlms/internal/conversion"
)

func promotionConvertHandler(d Dependencies) http.HandlerFunc {
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
			PreviewID  string `json:"previewId"`
			PositionID string `json:"positionId"`
			Scene      string `json:"scene"`
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
		result, err := d.Conversion.Convert(r.Context(), conversion.ConvertInput{
			OwnerUserID: userID, PreviewID: body.PreviewID, PositionID: body.PositionID,
			Scene: body.Scene, IdempotencyKey: key,
		})
		if err != nil {
			switch {
			case errors.Is(err, conversion.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "conversion request is invalid")
			case errors.Is(err, conversion.ErrNotEnabled):
				writeError(w, 403, "PROMOTER_NOT_ENABLED", "active promoter membership required")
			case errors.Is(err, conversion.ErrPosition), errors.Is(err, conversion.ErrPreview):
				writeError(w, 404, "PREVIEW_NOT_FOUND", "preview or position not found")
			case errors.Is(err, conversion.ErrNotReady):
				writeError(w, 409, "POSITION_NOT_READY", "channel position is not ready")
			case errors.Is(err, conversion.ErrExpired):
				writeError(w, 409, "PREVIEW_EXPIRED", "preview expired; refresh it before converting")
			case errors.Is(err, conversion.ErrPriceChanged):
				writeError(w, 409, "PRICE_CHANGED", "product quote changed; refresh the preview")
			case errors.Is(err, conversion.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to another conversion request")
			default:
				writeError(w, 503, "CHANNEL_UNAVAILABLE", "conversion service is unavailable")
			}
			return
		}
		if result.OwnerUserID != userID {
			writeError(w, 503, "CHANNEL_UNAVAILABLE", "conversion service is unavailable")
			return
		}
		status := 202
		message := "processing"
		if result.Status == "SUCCEEDED" || result.Status == "FAILED_FINAL" {
			status, message = 200, "completed"
		}
		writeJSON(w, status, map[string]any{"code": 0, "message": message, "data": conversionData(result)})
	}
}

func conversionData(r conversion.Record) map[string]any {
	data := map[string]any{
		"requestId": r.ID, "trackingId": r.TrackingID, "status": r.Status,
		"statusUrl": "/api/v1/promotions/convert/" + url.PathEscape(r.ID),
		"createdAt": r.CreatedAt, "updatedAt": r.UpdatedAt,
	}
	if r.Status == "SUCCEEDED" {
		data["linkUrl"] = r.LinkURL
		if r.SchemeURL != "" {
			data["schemeUrl"] = r.SchemeURL
		}
	}
	return data
}

func promotionConvertStatusHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
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
				writeError(w, 503, "CONVERSION_UNAVAILABLE", "conversion status is unavailable")
			}
			return
		}
		if record.OwnerUserID != userID || record.ID != r.PathValue("id") {
			writeError(w, 404, "CONVERSION_NOT_FOUND", "conversion request not found")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": conversionData(record)})
	}
}
