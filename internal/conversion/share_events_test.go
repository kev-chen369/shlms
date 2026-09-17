package conversion

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func shareEventDB(t *testing.T) *sql.DB {
	t.Helper()
	db := conversionDB(t)
	b, err := os.ReadFile("../../migrations/000010_promotion_share_events.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatal(err)
	}
	return db
}

func succeededShareRequest(t *testing.T, repo Repository, in ReserveInput) Record {
	t.Helper()
	ctx := context.Background()
	r, err := repo.Reserve(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	r, err = repo.Claim(ctx, r.ID, r.Version, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r, err = repo.MarkSucceeded(ctx, r.ID, r.ChannelRequestID, r.Version, "https://channel.example/item")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestShareEventConcurrentReplayConflictAndOwnerIsolation(t *testing.T) {
	repo := Repository{DB: shareEventDB(t)}
	r := succeededShareRequest(t, repo, request())
	in := ShareEventInput{OwnerUserID: "u1", LinkID: r.ID, EventID: "event1", Action: "copy_link", Scene: "group"}
	const workers = 12
	results := make([]ShareRecord, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = repo.RecordShareEvent(context.Background(), in) }(i)
	}
	wg.Wait()
	for i, got := range results {
		if errs[i] != nil || got.EventID != "event1" || got.LinkID != r.ID || got.TrackingID != r.TrackingID || got.Action != "copy_link" || got.Scene != "group" || got.RecordedAt.IsZero() || !got.RecordedAt.Equal(results[0].RecordedAt) {
			t.Fatal(i, got, errs[i])
		}
	}
	for _, change := range []func(*ShareEventInput){
		func(in *ShareEventInput) { in.Action = "copy_text" },
		func(in *ShareEventInput) { in.Scene = "social" },
	} {
		changed := in
		change(&changed)
		if _, err := repo.RecordShareEvent(context.Background(), changed); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatal(err)
		}
	}
	second := request()
	second.IdempotencyKey = "second-conversion"
	r2 := succeededShareRequest(t, repo, second)
	changed := in
	changed.LinkID = r2.ID
	if _, err := repo.RecordShareEvent(context.Background(), changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	other := request()
	other.OwnerUserID, other.PositionID, other.PreviewID, other.ChannelPositionID = "u2", "pos-u2", "pv-u2", "position-u2"
	r3 := succeededShareRequest(t, repo, other)
	otherEvent := in
	otherEvent.OwnerUserID, otherEvent.LinkID = "u2", r3.ID
	if got, err := repo.RecordShareEvent(context.Background(), otherEvent); err != nil || got.TrackingID != r3.TrackingID {
		t.Fatal(got, err)
	}
	// A fresh event ID is raced with two different operations: only the
	// winning six calls may replay, while the other six must conflict.
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mixed := in
			mixed.EventID = "event2"
			if i%2 == 1 {
				mixed.Action = "copy_text"
			}
			results[i], errs[i] = repo.RecordShareEvent(context.Background(), mixed)
		}(i)
	}
	wg.Wait()
	successes, conflicts := 0, 0
	var winner ShareRecord
	for i, got := range results {
		if errors.Is(errs[i], ErrIdempotencyConflict) {
			conflicts++
			continue
		}
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		successes++
		wantAction := "copy_link"
		if i%2 == 1 {
			wantAction = "copy_text"
		}
		if got.Action != wantAction {
			t.Fatal("receipt differs from caller input", i, got)
		}
		if winner.EventID == "" {
			winner = got
		}
		if got.EventID != "event2" || got.Action != winner.Action || !got.RecordedAt.Equal(winner.RecordedAt) {
			t.Fatal(got, winner)
		}
	}
	if successes != 6 || conflicts != 6 {
		t.Fatal(successes, conflicts)
	}
	for i, err := range errs {
		if !errors.Is(err, ErrIdempotencyConflict) {
			continue
		}
		wantAction := "copy_link"
		if i%2 == 1 {
			wantAction = "copy_text"
		}
		if wantAction == winner.Action {
			t.Fatal("winning operation incorrectly conflicted", i, winner)
		}
	}
	var events, tracking, requests int
	if err := repo.DB.QueryRow(`SELECT (SELECT count(*) FROM promotion_share_events),(SELECT count(*) FROM tracking_records),(SELECT count(*) FROM promotion_conversion_requests)`).Scan(&events, &tracking, &requests); err != nil || events != 3 || tracking != 3 || requests != 3 {
		t.Fatal(events, tracking, requests, err)
	}
	stored, err := repo.FindByID(context.Background(), "u1", r.ID)
	if err != nil || stored.Version != 3 || stored.AttemptCount != 1 || stored.Status != "SUCCEEDED" {
		t.Fatal(stored, err)
	}
}

func TestShareEventRejectsInvalidUnauthorizedAndPendingWithoutWrites(t *testing.T) {
	repo := Repository{DB: shareEventDB(t)}
	r, err := repo.Reserve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	in := ShareEventInput{OwnerUserID: "u1", LinkID: r.ID, EventID: "event1", Action: "copy_text", Scene: "group"}
	if _, err := repo.RecordShareEvent(context.Background(), in); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		change func(*ShareEventInput)
		want   error
	}{
		{func(in *ShareEventInput) { in.OwnerUserID = "u2" }, ErrNotFound},
		{func(in *ShareEventInput) { in.LinkID = "missing" }, ErrNotFound},
		{func(in *ShareEventInput) { in.EventID = "" }, ErrInvalid},
		{func(in *ShareEventInput) { in.Action = "delivered" }, ErrInvalid},
		{func(in *ShareEventInput) { in.Scene = "bad\x00scene" }, ErrInvalid},
	} {
		changed := in
		tc.change(&changed)
		if _, err := repo.RecordShareEvent(context.Background(), changed); !errors.Is(err, tc.want) {
			t.Fatal(err, tc.want)
		}
	}
	r, err = repo.Claim(context.Background(), r.ID, r.Version, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordShareEvent(context.Background(), in); !errors.Is(err, ErrStateConflict) {
		t.Fatal("processing event accepted", err)
	}
	r, err = repo.MarkUncertain(context.Background(), r.ID, r.ChannelRequestID, r.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordShareEvent(context.Background(), in); !errors.Is(err, ErrStateConflict) {
		t.Fatal("uncertain event accepted", err)
	}
	r, err = repo.MarkFinal(context.Background(), r.ID, r.ChannelRequestID, r.Version, "CHANNEL_REJECTED")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordShareEvent(context.Background(), in); !errors.Is(err, ErrStateConflict) {
		t.Fatal("failed event accepted", err)
	}
	var count int
	if err := repo.DB.QueryRow(`SELECT count(*) FROM promotion_share_events`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestShareEventInsertFailureRollsBack(t *testing.T) {
	repo := Repository{DB: shareEventDB(t)}
	r := succeededShareRequest(t, repo, request())
	if _, err := repo.DB.Exec(`CREATE FUNCTION reject_share() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END; $$;
		CREATE TRIGGER reject_share BEFORE INSERT ON promotion_share_events FOR EACH ROW EXECUTE FUNCTION reject_share()`); err != nil {
		t.Fatal(err)
	}
	in := ShareEventInput{OwnerUserID: "u1", LinkID: r.ID, EventID: "event1", Action: "copy_link", Scene: "group"}
	if _, err := repo.RecordShareEvent(context.Background(), in); err == nil {
		t.Fatal("injected failure accepted")
	}
	var count int
	if err := repo.DB.QueryRow(`SELECT count(*) FROM promotion_share_events`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if _, err := repo.DB.Exec(`DROP TRIGGER reject_share ON promotion_share_events`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordShareEvent(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

func TestShareEventMigrationRoundTripAndOwnershipConstraint(t *testing.T) {
	db := shareEventDB(t)
	repo := Repository{DB: db}
	r := succeededShareRequest(t, repo, request())
	if _, err := db.Exec(`INSERT INTO promotion_share_events(owner_user_id,event_id,conversion_id,action,scene) VALUES('u2','forged',$1,'copy_link','group')`, r.ID); err == nil {
		t.Fatal("cross-owner foreign key accepted")
	}
	for _, direction := range []string{"down", "up"} {
		b, err := os.ReadFile("../../migrations/000010_promotion_share_events." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.RecordShareEvent(context.Background(), ShareEventInput{OwnerUserID: "u1", LinkID: r.ID, EventID: "event1", Action: "copy_link", Scene: "group"}); err != nil {
		t.Fatal(err)
	}
}
