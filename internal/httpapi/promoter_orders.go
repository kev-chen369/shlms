package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kev-chen369/shlms/internal/order"
)

func orderReadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, order.ErrInvalid):
		writeError(w, 400, "INVALID_REQUEST", "order query is invalid")
	case errors.Is(err, order.ErrNotFound):
		writeError(w, 404, "ORDER_NOT_FOUND", "order not found")
	default:
		writeError(w, 503, "ORDERS_UNAVAILABLE", "orders are unavailable")
	}
}

func orderOwner(d Dependencies, w http.ResponseWriter, r *http.Request) string {
	owner, err := d.Users.ResolveUserID(r)
	if err != nil || strings.TrimSpace(owner) == "" {
		writeError(w, 401, "UNAUTHORIZED", "authentication required")
		return ""
	}
	return owner
}

func promoterOrderListHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner := orderOwner(d, w, r)
		if owner == "" {
			return
		}
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			orderReadError(w, order.ErrInvalid)
			return
		}
		for key, values := range q {
			if (key != "from" && key != "to" && key != "channel" && key != "positionId" && key != "orderStatus" && key != "cursor" && key != "limit") || len(values) != 1 {
				orderReadError(w, order.ErrInvalid)
				return
			}
		}
		filter := order.OrderFilter{OwnerUserID: owner, Channel: q.Get("channel"), PositionID: q.Get("positionId"), Status: q.Get("orderStatus"), Cursor: q.Get("cursor"), Limit: 20}
		if q.Has("limit") {
			filter.Limit, err = strconv.Atoi(q.Get("limit"))
			if err != nil {
				orderReadError(w, order.ErrInvalid)
				return
			}
		}
		if q.Has("from") {
			filter.From, err = time.Parse(time.RFC3339, q.Get("from"))
			if err != nil {
				orderReadError(w, order.ErrInvalid)
				return
			}
		}
		if q.Has("to") {
			filter.To, err = time.Parse(time.RFC3339, q.Get("to"))
			if err != nil {
				orderReadError(w, order.ErrInvalid)
				return
			}
		}
		if !filter.Valid() {
			orderReadError(w, order.ErrInvalid)
			return
		}
		page, err := d.Orders.ListOwned(r.Context(), filter)
		if err != nil {
			orderReadError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": page})
	}
}

func promoterOrderDetailHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner := orderOwner(d, w, r)
		if owner == "" {
			return
		}
		if r.URL.RawQuery != "" {
			orderReadError(w, order.ErrInvalid)
			return
		}
		item, err := d.Orders.GetOwned(r.Context(), owner, r.PathValue("id"))
		if err != nil {
			orderReadError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": item})
	}
}
