package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kev-chen369/shlms/internal/material"
)

func materialReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, material.ErrInvalid) {
		writeError(w, 400, "INVALID_REQUEST", "material query is invalid")
	} else {
		writeError(w, 503, "MATERIALS_UNAVAILABLE", "material catalog is unavailable")
	}
}

func materialReadQuery(r *http.Request, owner string, detail bool) (material.ReadInput, string, int, error) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return material.ReadInput{}, "", 0, material.ErrInvalid
	}
	for key, values := range q {
		switch key {
		case "platform", "type", "terminal", "cityCode", "business", "positionId", "scene":
		case "cursor", "limit":
			if detail {
				return material.ReadInput{}, "", 0, material.ErrInvalid
			}
		default:
			return material.ReadInput{}, "", 0, material.ErrInvalid
		}
		if len(values) != 1 {
			return material.ReadInput{}, "", 0, material.ErrInvalid
		}
	}
	in := material.ReadInput{OwnerID: owner, Scope: material.Context{Platform: q.Get("platform"), Type: q.Get("type"), Terminal: q.Get("terminal"), CityCode: q.Get("cityCode"), Business: q.Get("business")}, PositionID: q.Get("positionId"), Scene: q.Get("scene")}
	limit := 20
	if detail {
		limit = 1
	} else if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			return material.ReadInput{}, "", 0, material.ErrInvalid
		}
	}
	_, err = material.ParseQuery(material.Query{OwnerID: owner, Scope: in.Scope, Cursor: q.Get("cursor"), Limit: limit})
	if err != nil || !in.Valid() {
		return material.ReadInput{}, "", 0, material.ErrInvalid
	}
	return in, q.Get("cursor"), limit, nil
}

// Both routes are metadata-only. Media/config and all authorization checks
// remain server-owned; client filters never grant link generation permission.
func promoterMaterialHandler(d Dependencies, detail bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner := orderOwner(d, w, r)
		if owner == "" {
			return
		}
		in, cursor, limit, err := materialReadQuery(r, owner, detail)
		if err != nil {
			materialReadError(w, err)
			return
		}
		var data any
		if detail {
			data, err = d.Materials.Get(r.Context(), in, r.PathValue("id"))
		} else {
			data, err = d.Materials.List(r.Context(), in, cursor, limit)
		}
		if err != nil {
			materialReadError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": data})
	}
}
