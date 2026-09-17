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
			if (key != "platform" && key != "cityCode" && key != "business" && key != "cursor" && key != "limit") || len(values) != 1 {
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
		page, err := d.Coupons.List(r.Context(), coupon.ListInput{Platform: q.Get("platform"), CityCode: q.Get("cityCode"), Business: q.Get("business"), Cursor: q.Get("cursor"), Limit: limit})
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

func couponContext(r *http.Request, withPage bool) (string, string, string, int, error) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", "", "", 0, coupon.ErrInvalid
	}
	for key, values := range q {
		if (key != "cityCode" && key != "business" && (!withPage || key != "cursor") && (!withPage || key != "limit")) || len(values) != 1 {
			return "", "", "", 0, coupon.ErrInvalid
		}
	}
	limit := 20
	if withPage && q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			return "", "", "", 0, coupon.ErrInvalid
		}
	}
	return q.Get("cityCode"), q.Get("business"), q.Get("cursor"), limit, nil
}

func couponReadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, coupon.ErrInvalid):
		writeError(w, 400, "INVALID_REQUEST", "coupon request is invalid")
	case errors.Is(err, coupon.ErrNotFound):
		writeError(w, 404, "COUPON_NOT_FOUND", "coupon not found")
	default:
		writeError(w, 503, "COUPONS_UNAVAILABLE", "coupon catalog is unavailable")
	}
}

func couponDetailHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		city, business, _, _, err := couponContext(r, false)
		if err != nil {
			couponReadError(w, err)
			return
		}
		item, err := d.Coupons.Get(r.Context(), r.PathValue("id"), city, business)
		if err != nil {
			couponReadError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": item})
	}
}

func couponProductsHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		city, business, cursor, limit, err := couponContext(r, true)
		if err != nil {
			couponReadError(w, err)
			return
		}
		page, err := d.Coupons.Products(r.Context(), r.PathValue("id"), city, business, cursor, limit)
		if err != nil {
			couponReadError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": page})
	}
}
