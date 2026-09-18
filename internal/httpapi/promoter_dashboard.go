package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/kev-chen369/shlms/internal/dashboard"
)

func promoterDashboardHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner := orderOwner(d, w, r)
		if owner == "" {
			return
		}
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			writeError(w, 400, "INVALID_REQUEST", "dashboard query is invalid")
			return
		}
		for key, values := range q {
			if (key != "from" && key != "to" && key != "channel" && key != "positionId") || len(values) != 1 {
				writeError(w, 400, "INVALID_REQUEST", "dashboard query is invalid")
				return
			}
		}
		zone, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			writeError(w, 503, "DASHBOARD_UNAVAILABLE", "dashboard is unavailable")
			return
		}
		today := time.Now().In(zone)
		startOfToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, zone)
		from := startOfToday.AddDate(0, 0, -29)
		to := startOfToday.AddDate(0, 0, 1)
		if q.Has("from") {
			from, err = time.ParseInLocation("2006-01-02", q.Get("from"), zone)
			if err != nil || from.Format("2006-01-02") != q.Get("from") {
				writeError(w, 400, "INVALID_REQUEST", "dashboard query is invalid")
				return
			}
		}
		if q.Has("to") {
			end, parseErr := time.ParseInLocation("2006-01-02", q.Get("to"), zone)
			if parseErr != nil || end.Format("2006-01-02") != q.Get("to") {
				writeError(w, 400, "INVALID_REQUEST", "dashboard query is invalid")
				return
			}
			to = end.AddDate(0, 0, 1)
		}
		filter := dashboard.Filter{OwnerUserID: owner, Channel: q.Get("channel"), PositionID: q.Get("positionId"), From: from, To: to}
		if !filter.Valid() {
			writeError(w, 400, "INVALID_REQUEST", "dashboard query is invalid")
			return
		}
		counts, err := d.Dashboard.Get(r.Context(), filter)
		if err != nil {
			if errors.Is(err, dashboard.ErrInvalid) {
				writeError(w, 400, "INVALID_REQUEST", "dashboard query is invalid")
			} else {
				writeError(w, 503, "DASHBOARD_UNAVAILABLE", "dashboard is unavailable")
			}
			return
		}
		// UTC wire timestamps preserve historical second-level timezone offsets.
		// The filter and declared calendar zone remain Asia/Shanghai.
		counts.From = counts.From.UTC()
		counts.ToExclusive = counts.ToExclusive.UTC()
		counts.AsOf = counts.AsOf.UTC()
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": counts})
	}
}
