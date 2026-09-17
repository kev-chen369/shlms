package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/coupon"
)

func couponOutboundHandler(d Dependencies) http.HandlerFunc {
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
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			writeError(w, 400, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
			return
		}
		var body struct {
			CityCode   string `json:"cityCode"`
			Business   string `json:"business"`
			Terminal   string `json:"terminal"`
			EntryPoint string `json:"entryPoint"`
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
				err = errors.New("multiple values")
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
		if (body.Terminal != "H5" && body.Terminal != "APP" && body.Terminal != "WECHAT") || strings.TrimSpace(body.EntryPoint) == "" {
			writeError(w, 400, "INVALID_REQUEST", "outbound request is invalid")
			return
		}
		result, err := d.Outbound.Prepare(r.Context(), coupon.OutboundInput{
			OwnerKey: "user:" + owner, CouponID: r.PathValue("id"), CityCode: body.CityCode,
			Business: body.Business, Terminal: body.Terminal, EntryPoint: body.EntryPoint, IdempotencyKey: key,
		})
		if err != nil {
			switch {
			case errors.Is(err, coupon.ErrOutboundInvalid), errors.Is(err, coupon.ErrInvalid):
				writeError(w, 400, "INVALID_REQUEST", "outbound request is invalid")
			case errors.Is(err, coupon.ErrNotFound):
				writeError(w, 404, "COUPON_NOT_FOUND", "coupon not found")
			case errors.Is(err, coupon.ErrOutboundConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to another outbound request")
			default:
				writeError(w, 503, "OUTBOUND_UNAVAILABLE", "outbound target is unavailable")
			}
			return
		}
		if result.CouponID != r.PathValue("id") || result.ID == "" || result.TargetURL == "" {
			writeError(w, 503, "OUTBOUND_UNAVAILABLE", "outbound target is unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{
			"outboundId": result.ID, "couponId": result.CouponID, "platform": result.Platform,
			"targetUrl": result.TargetURL, "status": "READY_TO_OPEN", "createdAt": result.CreatedAt,
		}})
	}
}
