package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kev-chen369/shlms/internal/coupon"
)

func couponListHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			writeError(w, 400, "INVALID_REQUEST", "coupon query is invalid")
			return
		}
		for key, values := range q {
			if (key != "platform" && key != "cursor" && key != "limit") || len(values) != 1 {
				writeError(w, 400, "INVALID_REQUEST", "coupon query is invalid")
				return
			}
		}
		limit := 20
		if _, ok := q["limit"]; ok {
			limit, err = strconv.Atoi(q.Get("limit"))
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, 400, "INVALID_REQUEST", "coupon query is invalid")
				return
			}
		}
		page, err := d.Coupons.List(r.Context(), coupon.ListInput{Platform: q.Get("platform"), Cursor: q.Get("cursor"), Limit: limit})
		if err != nil {
			if errors.Is(err, coupon.ErrInvalid) {
				writeError(w, 400, "INVALID_REQUEST", "coupon query is invalid")
			} else {
				writeError(w, 503, "COUPONS_UNAVAILABLE", "coupon catalog is unavailable")
			}
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": page})
	}
}
