package conversion

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode"
	"unicode/utf8"
)

type ShareEventInput struct {
	OwnerUserID string
	LinkID      string
	EventID     string
	Action      string
	Scene       string
}

type ShareRecord struct {
	EventID    string
	LinkID     string
	TrackingID string
	Action     string
	Scene      string
	RecordedAt time.Time
}

func (r Repository) RecordShareEvent(ctx context.Context, in ShareEventInput) (ShareRecord, error) {
	if !validText(in.OwnerUserID, 128) || !validText(in.LinkID, 128) || !validText(in.EventID, 128) || !validText(in.Scene, 80) || (in.Action != "copy_link" && in.Action != "copy_text") {
		return ShareRecord{}, ErrInvalid
	}
	for _, value := range []string{in.OwnerUserID, in.LinkID, in.EventID, in.Scene} {
		if !utf8.ValidString(value) {
			return ShareRecord{}, ErrInvalid
		}
		for _, ch := range value {
			if unicode.IsControl(ch) {
				return ShareRecord{}, ErrInvalid
			}
		}
	}
	if r.DB == nil {
		return ShareRecord{}, ErrUnavailable
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return ShareRecord{}, err
	}
	defer tx.Rollback()
	out := ShareRecord{EventID: in.EventID, LinkID: in.LinkID, Action: in.Action, Scene: in.Scene}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT tracking_id,status FROM promotion_conversion_requests WHERE id=$1 AND owner_user_id=$2 FOR SHARE`, in.LinkID, in.OwnerUserID).Scan(&out.TrackingID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ShareRecord{}, ErrNotFound
	}
	if err != nil {
		return ShareRecord{}, err
	}
	if status != "SUCCEEDED" {
		return ShareRecord{}, ErrStateConflict
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO promotion_share_events(owner_user_id,event_id,conversion_id,action,scene)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_user_id,event_id) DO NOTHING RETURNING recorded_at`, in.OwnerUserID, in.EventID, in.LinkID, in.Action, in.Scene).Scan(&out.RecordedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var linkID, action, scene string
		err = tx.QueryRowContext(ctx, `SELECT conversion_id,action,scene,recorded_at FROM promotion_share_events WHERE owner_user_id=$1 AND event_id=$2`, in.OwnerUserID, in.EventID).Scan(&linkID, &action, &scene, &out.RecordedAt)
		if err != nil {
			return ShareRecord{}, err
		}
		if linkID != in.LinkID || action != in.Action || scene != in.Scene {
			return ShareRecord{}, ErrIdempotencyConflict
		}
	} else if err != nil {
		return ShareRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ShareRecord{}, err
	}
	return out, nil
}
