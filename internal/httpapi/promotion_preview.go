package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/preview"
)

func promotionPreviewHandler(d Dependencies) http.HandlerFunc {
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
			Input      string `json:"input"`
			PositionID string `json:"positionId"`
			Scene      string `json:"scene"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
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
		got, err := d.Preview.Create(r.Context(), preview.Request{
			OwnerUserID: userID, PositionID: body.PositionID, Scene: body.Scene,
			Input: body.Input, IdempotencyKey: key,
		})
		if err != nil {
			switch {
			case errors.Is(err, preview.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "preview request is invalid")
			case errors.Is(err, preview.ErrNotEnabled):
				writeError(w, 403, "PROMOTER_NOT_ENABLED", "active promoter membership required")
			case errors.Is(err, preview.ErrPosition):
				writeError(w, 404, "POSITION_NOT_FOUND", "position not found")
			case errors.Is(err, preview.ErrNotReady):
				writeError(w, 409, "POSITION_NOT_READY", "channel position is not ready")
			case errors.Is(err, preview.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to a different preview request")
			default:
				writeError(w, 503, "CHANNEL_UNAVAILABLE", "preview quote is unavailable")
			}
			return
		}
		if got.OwnerUserID != userID {
			writeError(w, 503, "CHANNEL_UNAVAILABLE", "preview quote is unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{
			"previewId": got.ID, "positionId": got.PositionID, "scene": got.Scene,
			"product":  map[string]any{"channel": got.Channel, "externalProductId": got.ExternalProductID, "name": got.ProductName},
			"currency": got.Currency, "couponPriceMinor": got.CouponPriceMinor,
			"promoterEstimateMinor":         got.PromoterEstimateMinor,
			"consumerCashbackEstimateMinor": got.ConsumerCashbackEstimateMinor,
			"ruleVersion":                   got.RuleVersion, "updatedAt": got.UpdatedAt, "expiresAt": got.ExpiresAt,
		}})
	}
}
