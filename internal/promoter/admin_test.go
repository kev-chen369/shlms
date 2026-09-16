package promoter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

type adminWriterFunc func(context.Context, string, AdminCommand) (Profile, error)

func (f adminWriterFunc) ExecuteAdmin(ctx context.Context, id string, c AdminCommand) (Profile, error) {
	return f(ctx, id, c)
}
func reviewCommand() AdminCommand {
	return AdminCommand{Action: ReviewAction, TargetID: "app-1", IdempotencyKey: "review-key", Approve: true, Reason: "approved", Version: 1}
}
func adminActor() AdminActor {
	return AdminActor{ID: "admin", Permissions: map[string]bool{ReviewPermission: true, DisablePermission: true}}
}

func TestAdminServiceAuthorization(t *testing.T) {
	called := 0
	s := AdminService{Repository: adminWriterFunc(func(ctx context.Context, id string, c AdminCommand) (Profile, error) { called++; return Profile{}, nil })}
	for _, actor := range []AdminActor{{}, {ID: "u1"}, {ID: "admin", Permissions: map[string]bool{DisablePermission: true}}} {
		if _, err := s.Execute(context.Background(), actor, reviewCommand()); !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
	}
	if called != 0 {
		t.Fatal("unauthorized repository call")
	}
	c := reviewCommand()
	c.Reason = " "
	if _, err := s.Execute(context.Background(), adminActor(), c); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err := s.Execute(context.Background(), adminActor(), reviewCommand()); err != nil || called != 1 {
		t.Fatal(err, called)
	}
}

func TestPostgresAdminReviewConcurrencyAndAudit(t *testing.T) {
	db := promoterDB(t)
	migration, err := os.ReadFile("../../migrations/000003_promoter_admin_audits.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(db)
	s := AdminService{Repository: repo}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err = repo.Submit(ctx, "apply", application()); err != nil {
		t.Fatal(err)
	}
	const n = 12
	out := make([]Profile, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func(i int) { defer wg.Done(); out[i], errs[i] = s.Execute(ctx, adminActor(), reviewCommand()) }(i)
	}
	wg.Wait()
	for i := range out {
		if errs[i] != nil || out[i].Status != Enabled || out[i].Version != 2 {
			t.Fatal(i, out[i], errs[i])
		}
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM promoter_admin_audits`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	c := reviewCommand()
	c.Approve = false
	if _, err = s.Execute(ctx, adminActor(), c); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	c = AdminCommand{Action: DisableAction, TargetID: "u1", IdempotencyKey: "stop", Reason: "policy", Version: 2}
	p, err := s.Execute(ctx, adminActor(), c)
	if err != nil || p.Status != Disabled || p.Version != 3 {
		t.Fatal(p, err)
	}
	old, err := s.Execute(ctx, adminActor(), reviewCommand())
	if err != nil || old.Status != Enabled || old.Version != 2 {
		t.Fatal("replay not original", old, err)
	}
	p, err = repo.FindByUserID(ctx, "u1")
	if err != nil || p.Status != Disabled {
		t.Fatal("replay changed state", p, err)
	}
	if _, err = db.Exec(`UPDATE promoter_admin_audits SET actor_id='tampered'`); err == nil {
		t.Fatal("audit changed")
	}
	if _, err = db.Exec(`DELETE FROM promoter_admin_audits`); err == nil {
		t.Fatal("audit deleted")
	}
	var status Status
	if err = db.QueryRow(`SELECT status FROM promoter_applications WHERE id='app-1'`).Scan(&status); err != nil || status != Enabled {
		t.Fatal(status, err)
	}
}

func TestPostgresAdminConflictsAndRollback(t *testing.T) {
	db := promoterDB(t)
	migration, err := os.ReadFile("../../migrations/000003_promoter_admin_audits.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(db)
	s := AdminService{Repository: repo}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err = repo.Submit(ctx, "apply", application()); err != nil {
		t.Fatal(err)
	}
	actor := adminActor()
	actor.ID = "u1"
	if _, err = s.Execute(ctx, actor, reviewCommand()); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	const n = 12
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := reviewCommand()
			c.IdempotencyKey = fmt.Sprintf("key-%d", i)
			_, errs[i] = s.Execute(ctx, adminActor(), c)
		}(i)
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatal(wins)
	}
	// Force audit insertion failure after state update; the whole transaction must roll back.
	if _, err = db.Exec(`ALTER TABLE promoter_admin_audits ADD CONSTRAINT test_reject_disable CHECK(action <> 'DISABLE')`); err != nil {
		t.Fatal(err)
	}
	c := AdminCommand{Action: DisableAction, TargetID: "u1", IdempotencyKey: "stop", Reason: "policy", Version: 2}
	if _, err = s.Execute(ctx, adminActor(), c); err == nil {
		t.Fatal("audit failure accepted")
	}
	p, err := repo.FindByUserID(ctx, "u1")
	if err != nil || p.Status != Enabled || p.Version != 2 {
		t.Fatal("state survived failed audit", p, err)
	}
	down, err := os.ReadFile("../../migrations/000003_promoter_admin_audits.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(down)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
}
