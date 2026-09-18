package conversion

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

var shareEventID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var shareScene = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

type ShareEventInput struct {
	OwnerUserID  string
	RequestID    string
	EventID      string
	ArtifactType string
	Scene        string
	Action       string
}

type ShareEvent struct {
	EventID      string    `json:"eventId"`
	RequestID    string    `json:"requestId"`
	Action       string    `json:"action"`
	ArtifactType string    `json:"artifactType"`
	Scene        string    `json:"scene"`
	RecordedAt   time.Time `json:"recordedAt"`
}

type ShareEventStore struct{ DB *sql.DB }

// Record stores a client report, not evidence that the clipboard was written or a message delivered.
func (s ShareEventStore) Record(ctx context.Context, in ShareEventInput) (ShareEvent, error) {
	if s.DB == nil || !validText(in.OwnerUserID, 128) || !validText(in.RequestID, 128) || !shareEventID.MatchString(in.EventID) || !shareScene.MatchString(in.Scene) || in.Action != "COPY_REPORTED" || (in.ArtifactType != "link" && in.ArtifactType != "text") {
		return ShareEvent{}, ErrInvalid
	}
	var status string
	err := s.DB.QueryRowContext(ctx, `SELECT status FROM promotion_conversion_requests WHERE id=$1 AND owner_user_id=$2`, in.RequestID, in.OwnerUserID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ShareEvent{}, ErrNotFound
	}
	if err != nil {
		return ShareEvent{}, err
	}
	if status != "SUCCEEDED" {
		return ShareEvent{}, ErrShareNotReady
	}
	storedAction := "copy_" + in.ArtifactType
	var recordedAt time.Time
	err = s.DB.QueryRowContext(ctx, `INSERT INTO promotion_share_events(owner_user_id,event_id,conversion_id,action,scene)
  VALUES($1,$2,$3,$4,$5) ON CONFLICT (owner_user_id,event_id) DO NOTHING RETURNING recorded_at`, in.OwnerUserID, in.EventID, in.RequestID, storedAction, in.Scene).Scan(&recordedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var existing ShareEvent
		err = s.DB.QueryRowContext(ctx, `SELECT conversion_id,action,scene,recorded_at FROM promotion_share_events WHERE owner_user_id=$1 AND event_id=$2`, in.OwnerUserID, in.EventID).Scan(&existing.RequestID, &existing.Action, &existing.Scene, &existing.RecordedAt)
		if err != nil {
			return ShareEvent{}, err
		}
		if existing.RequestID != in.RequestID || existing.Action != storedAction || existing.Scene != in.Scene {
			return ShareEvent{}, ErrIdempotencyConflict
		}
		recordedAt = existing.RecordedAt
	} else if err != nil {
		return ShareEvent{}, err
	}
	return ShareEvent{EventID: in.EventID, RequestID: in.RequestID, Action: in.Action, ArtifactType: in.ArtifactType, Scene: in.Scene, RecordedAt: recordedAt}, nil
}
