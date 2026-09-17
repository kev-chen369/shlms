package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/kev-chen369/shlms/internal/coupon"
)

func claimError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, coupon.ErrClaimInvalid):
		writeError(w, 400, "INVALID_REQUEST", "claim request is invalid")
	case errors.Is(err, coupon.ErrClaimConflict):
		writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to another claim request")
	case errors.Is(err, coupon.ErrClaimNotFound), errors.Is(err, coupon.ErrNotFound):
		writeError(w, 404, "COUPON_OR_CLAIM_NOT_FOUND", "coupon or claim not found")
	case errors.Is(err, coupon.ErrNotClaimable):
		writeError(w, 409, "COUPON_NOT_CLAIMABLE", "coupon must be claimed on the platform")
	default:
		writeError(w, 503, "CLAIM_UNAVAILABLE", "claim service is unavailable")
	}
}

func claimData(c coupon.Claim) map[string]any {
	return map[string]any{"claimId": c.ID, "couponId": c.CouponID, "status": c.Status,
		"statusUrl": "/api/v1/coupon-claims/" + url.PathEscape(c.ID), "createdAt": c.CreatedAt, "updatedAt": c.UpdatedAt}
}

func couponClaimHandler(d Dependencies) http.HandlerFunc {
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
			CityCode string `json:"cityCode"`
			Business string `json:"business"`
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
		claim, err := d.Claims.Create(r.Context(), coupon.ClaimInput{OwnerUserID: owner, CouponID: r.PathValue("id"),
			CityCode: body.CityCode, Business: body.Business, IdempotencyKey: key})
		if err != nil {
			claimError(w, err)
			return
		}
		if claim.OwnerUserID != owner {
			claimError(w, errors.New("owner mismatch"))
			return
		}
		status := 202
		if claim.Status == "CLAIMED" || claim.Status == "FAILED" {
			status = 200
		}
		writeJSON(w, status, map[string]any{"code": 0, "message": "success", "data": claimData(claim)})
	}
}

func couponClaimStatusHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(owner) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		claim, err := d.ClaimReader.Get(r.Context(), owner, r.PathValue("id"))
		if err != nil {
			claimError(w, err)
			return
		}
		if claim.OwnerUserID != owner {
			claimError(w, errors.New("owner mismatch"))
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": claimData(claim)})
	}
}
