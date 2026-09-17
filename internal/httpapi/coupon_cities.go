package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/kev-chen369/shlms/internal/coupon"
)

func couponCitiesHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(q) != 1 || len(q["platform"]) != 1 {
			writeError(w, 400, "INVALID_REQUEST", "platform is required")
			return
		}
		cities, err := d.CouponCities.Cities(r.Context(), q.Get("platform"))
		if err != nil {
			if errors.Is(err, coupon.ErrInvalid) {
				writeError(w, 400, "INVALID_REQUEST", "platform is invalid")
			} else {
				writeError(w, 503, "COUPONS_UNAVAILABLE", "coupon cities are unavailable")
			}
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{"items": cities}})
	}
}
