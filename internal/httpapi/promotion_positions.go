package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type PositionManager interface {
	ChangePosition(context.Context, string, promoter.PositionCommand) (promoter.Position, error)
	ListPositions(context.Context, string, promoter.PositionListInput) (promoter.PositionPage, error)
}

func positionView(p promoter.Position) map[string]any {
	readiness := p.ChannelReadiness
	if readiness == "" {
		readiness = "WAITING_CONFIGURATION"
	}
	return map[string]any{"id": p.ID, "name": p.Name, "scene": p.Scene, "status": p.Status, "isDefault": p.IsDefault, "version": p.Version, "createdAt": p.CreatedAt,
		"canConvert": false, "channels": []map[string]string{{"channel": "JD", "readiness": readiness}}}
}

func positionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, promoter.ErrInvalidInput):
		writeError(w, 400, "INVALID_REQUEST", "position request is invalid")
	case errors.Is(err, promoter.ErrNotEnabled):
		writeError(w, 403, "PROMOTER_NOT_ENABLED", "active promoter membership required")
	case errors.Is(err, promoter.ErrNotFound):
		writeError(w, 404, "POSITION_NOT_FOUND", "position not found")
	case errors.Is(err, promoter.ErrConflict):
		writeError(w, 409, "VERSION_CONFLICT", "refresh the position before retrying")
	case errors.Is(err, promoter.ErrIdempotencyConflict):
		writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to a different position request")
	case errors.Is(err, promoter.ErrTransition):
		writeError(w, 409, "POSITION_DISABLED", "disabled positions cannot be modified")
	default:
		writeError(w, 503, "POSITIONS_UNAVAILABLE", "position service is unavailable")
	}
}

func positionWriteHandler(d Dependencies, action string) http.HandlerFunc {
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
			Name      *string `json:"name"`
			Scene     *string `json:"scene"`
			IsDefault *bool   `json:"isDefault"`
			Version   *int64  `json:"version"`
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
		c := promoter.PositionCommand{Action: action, ID: r.PathValue("id"), IdempotencyKey: key}
		valid := false
		switch action {
		case promoter.PositionCreate:
			valid = body.Name != nil && body.Scene != nil && body.Version == nil
		case promoter.PositionEdit:
			valid = body.Name != nil && body.Scene != nil && body.Version != nil && body.IsDefault == nil
		case promoter.PositionDefault, promoter.PositionDisable:
			valid = body.Version != nil && body.Name == nil && body.Scene == nil && body.IsDefault == nil
		}
		if !valid {
			writeError(w, 400, "INVALID_REQUEST", "fields do not match the requested position action")
			return
		}
		if body.Name != nil {
			c.Name = *body.Name
		}
		if body.Scene != nil {
			c.Scene = *body.Scene
		}
		if body.IsDefault != nil {
			c.IsDefault = *body.IsDefault
		}
		if body.Version != nil {
			c.Version = *body.Version
		}
		p, err := d.Positions.ChangePosition(r.Context(), userID, c)
		if err != nil {
			positionError(w, err)
			return
		}
		if p.OwnerUserID != userID {
			positionError(w, errors.New("owner mismatch"))
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": positionView(p)})
	}
}

func positionListHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			positionError(w, promoter.ErrInvalidInput)
			return
		}
		for key, values := range q {
			if (key != "status" && key != "cursor" && key != "limit") || len(values) != 1 {
				positionError(w, promoter.ErrInvalidInput)
				return
			}
		}
		limit := 20
		if values, ok := q["limit"]; ok {
			limit, err = strconv.Atoi(values[0])
			if err != nil || limit < 1 || limit > 100 {
				positionError(w, promoter.ErrInvalidInput)
				return
			}
		}
		page, err := d.Positions.ListPositions(r.Context(), userID, promoter.PositionListInput{Status: promoter.Status(q.Get("status")), Cursor: q.Get("cursor"), Limit: limit})
		if err != nil {
			positionError(w, err)
			return
		}
		items := []map[string]any{}
		for _, p := range page.Items {
			if p.OwnerUserID != userID {
				positionError(w, errors.New("owner mismatch"))
				return
			}
			items = append(items, positionView(p))
		}
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{"items": items, "nextCursor": page.NextCursor}})
	}
}
