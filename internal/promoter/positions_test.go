package promoter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func positionsDB(t *testing.T) *sql.DB {
	t.Helper()
	db := promoterDB(t)
	for _, name := range []string{"000003_promoter_admin_audits.up.sql", "000004_promotion_positions.up.sql", "000005_channel_positions.up.sql"} {
		b, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPostgresRepository(db)
	if _, err := repo.Submit(context.Background(), "apply", application()); err != nil {
		t.Fatal(err)
	}
	if _, err := (AdminService{Repository: repo}).Execute(context.Background(), adminActor(), reviewCommand()); err != nil {
		t.Fatal(err)
	}
	return db
}
func createPosition(key string) PositionCommand {
	return PositionCommand{Action: PositionCreate, Name: "好物群", Scene: "群分享", IdempotencyKey: key}
}

func TestPostgresPositionsConcurrentCreation(t *testing.T) {
	db := positionsDB(t)
	s := PositionService{Repository: NewPostgresRepository(db)}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const n = 12
	out := make([]Position, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func(i int) { defer wg.Done(); out[i], errs[i] = s.ChangePosition(ctx, "u1", createPosition("same")) }(i)
	}
	wg.Wait()
	for i := range out {
		if errs[i] != nil || out[i].ID != out[0].ID || out[i].OwnerUserID != "u1" {
			t.Fatal(i, out[i], errs[i])
		}
	}
	firstID := out[0].ID
	c := createPosition("same")
	c.Name = "different"
	if _, err := s.ChangePosition(ctx, "u1", c); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	for i := range out {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := createPosition(fmt.Sprintf("default-%d", i))
			c.IsDefault = true
			out[i], errs[i] = s.ChangePosition(ctx, "u1", c)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var defaults, total int
	if err := db.QueryRow(`SELECT count(*) FILTER(WHERE is_default),count(*) FROM promotion_positions`).Scan(&defaults, &total); err != nil || defaults != 1 || total != 13 {
		t.Fatal(defaults, total, err)
	}
	// A command replay is an immutable receipt, not the current default state.
	replay, err := s.ChangePosition(ctx, "u1", createPosition("same"))
	if err != nil || replay.ID != firstID || replay.Name != "好物群" {
		t.Fatal(replay, err)
	}
}

func TestPostgresConcurrentDefaultSwitch(t *testing.T) {
	db := positionsDB(t)
	s := PositionService{Repository: NewPostgresRepository(db)}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const n = 12
	positions := make([]Position, n)
	for i := range positions {
		p, err := s.ChangePosition(ctx, "u1", createPosition(fmt.Sprintf("create-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		positions[i] = p
	}
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range positions {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = s.ChangePosition(ctx, "u1", PositionCommand{Action: PositionDefault, ID: positions[i].ID, Version: 1, IdempotencyKey: fmt.Sprintf("default-%d", i)})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM promotion_positions WHERE is_default`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	page, err := s.ListPositions(ctx, "u1", PositionListInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range page.Items {
		want := int64(3)
		if p.IsDefault {
			want = 2
		}
		if p.Version != want {
			t.Fatal("default changes must invalidate stale editors", p)
		}
	}
}

func TestPostgresPositionLifecycleAndIsolation(t *testing.T) {
	db := positionsDB(t)
	repo := NewPostgresRepository(db)
	s := PositionService{Repository: repo}
	ctx := context.Background()
	c := createPosition("one")
	c.IsDefault = true
	p, err := s.ChangePosition(ctx, "u1", c)
	if err != nil {
		t.Fatal(err)
	}
	a := application()
	a.ID = "app-2"
	a.UserID = "u2"
	if _, err = repo.Submit(ctx, "apply", a); err != nil {
		t.Fatal(err)
	}
	review := reviewCommand()
	review.TargetID = a.ID
	review.IdempotencyKey = "review-2"
	if _, err = (AdminService{Repository: repo}).Execute(ctx, adminActor(), review); err != nil {
		t.Fatal(err)
	}
	edit := PositionCommand{Action: PositionEdit, ID: p.ID, Name: "new", Scene: "new", Version: 1, IdempotencyKey: "edit"}
	if _, err = s.ChangePosition(ctx, "u2", edit); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross user edit", err)
	}
	p, err = s.ChangePosition(ctx, "u1", edit)
	if err != nil || p.Version != 2 || !p.IsDefault {
		t.Fatal(p, err)
	}
	edit.IdempotencyKey = "stale"
	if _, err = s.ChangePosition(ctx, "u1", edit); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	stop := PositionCommand{Action: PositionDisable, ID: p.ID, Version: 2, IdempotencyKey: "stop"}
	p, err = s.ChangePosition(ctx, "u1", stop)
	if err != nil || p.Status != Disabled || p.IsDefault || p.Version != 3 {
		t.Fatal(p, err)
	}
	def := PositionCommand{Action: PositionDefault, ID: p.ID, Version: 3, IdempotencyKey: "default"}
	if _, err = s.ChangePosition(ctx, "u1", def); !errors.Is(err, ErrTransition) {
		t.Fatal(err)
	}
	page, err := s.ListPositions(ctx, "u1", PositionListInput{Status: Disabled})
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	page, err = s.ListPositions(ctx, "u2", PositionListInput{})
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	if _, err = (AdminService{Repository: repo}).Execute(ctx, adminActor(), AdminCommand{Action: DisableAction, TargetID: "u1", Reason: "policy", Version: 2, IdempotencyKey: "disable-user"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangePosition(ctx, "u1", createPosition("blocked")); !errors.Is(err, ErrNotEnabled) {
		t.Fatal(err)
	}
	page, err = s.ListPositions(ctx, "u1", PositionListInput{})
	if err != nil || len(page.Items) != 1 {
		t.Fatal("lost history", page, err)
	}
}

func TestPostgresPositionPaginationAndRollback(t *testing.T) {
	db := positionsDB(t)
	s := PositionService{Repository: NewPostgresRepository(db)}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := s.ChangePosition(ctx, "u1", createPosition(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListPositions(ctx, "u1", PositionListInput{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	next, err := s.ListPositions(ctx, "u1", PositionListInput{Limit: 2, Cursor: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.NextCursor != "" || next.Items[0].ID == page.Items[1].ID {
		t.Fatal(next, err)
	}
	if _, err = s.ListPositions(ctx, "u2", PositionListInput{Cursor: page.NextCursor}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err = db.Exec(`ALTER TABLE promotion_position_requests ADD CONSTRAINT force_failure CHECK(action <> 'DEFAULT')`); err != nil {
		t.Fatal(err)
	}
	p := page.Items[0]
	if _, err = s.ChangePosition(ctx, "u1", PositionCommand{Action: PositionDefault, ID: p.ID, Version: p.Version, IdempotencyKey: "fail"}); err == nil {
		t.Fatal("receipt failure accepted")
	}
	var defaults int
	if err = db.QueryRow(`SELECT count(*) FROM promotion_positions WHERE is_default`).Scan(&defaults); err != nil || defaults != 0 {
		t.Fatal(defaults, err)
	}
	for _, name := range []string{"000005_channel_positions.down.sql", "000004_promotion_positions.down.sql", "000004_promotion_positions.up.sql", "000005_channel_positions.up.sql"} {
		b, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
}
