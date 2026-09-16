package promoter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func promoterDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN is not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	schema := fmt.Sprintf("promoter_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	applyMigration(t, db, "up")
	return db
}

func applyMigration(t *testing.T, db *sql.DB, direction string) {
	t.Helper()
	b, err := os.ReadFile("../../migrations/000002_promoter_applications." + direction + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresApplicationConcurrencyAndIsolation(t *testing.T) {
	db := promoterDB(t)
	repo := NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const n = 12
	out := make([]Application, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := application()
			a.ID = fmt.Sprintf("app-%d", i)
			out[i], errs[i] = repo.Submit(ctx, "same-key", a)
		}(i)
	}
	wg.Wait()
	for i := range out {
		if errs[i] != nil || out[i].ID != out[0].ID {
			t.Fatalf("caller %d: %+v %v", i, out[i], errs[i])
		}
	}
	p, err := repo.FindByUserID(ctx, "u1")
	if err != nil || p.Status != Pending || p.Version != 1 || p.ApplicationID != out[0].ID {
		t.Fatal(p, err)
	}
	a := application()
	a.Scene = "other"
	if _, err = repo.Submit(ctx, "same-key", a); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	a = application()
	if _, err = repo.Submit(ctx, "different-key", a); !errors.Is(err, ErrTransition) {
		t.Fatal(err)
	}
	a.UserID = "u2"
	a.ID = "u2-app"
	if _, err = repo.Submit(ctx, "same-key", a); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.FindByUserID(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM promoter_applications WHERE user_id='u1'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestPostgresDifferentKeysAndRollback(t *testing.T) {
	db := promoterDB(t)
	repo := NewPostgresRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const n = 12
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := application()
			a.ID = fmt.Sprintf("unique-%d", i)
			_, errs[i] = repo.Submit(ctx, fmt.Sprintf("key-%d", i), a)
		}(i)
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrTransition) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatal(wins)
	}
	a := application()
	a.UserID = "rollback-user"
	a.ID = "rollback-app"
	a.DisplayName = strings.Repeat("x", 81)
	if _, err := repo.Submit(ctx, "rollback-key", a); err == nil {
		t.Fatal("expected database error")
	}
	if _, err := repo.FindByUserID(ctx, a.UserID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("profile survived rollback: %v", err)
	}
	applyMigration(t, db, "down")
	applyMigration(t, db, "up")
	if _, err := repo.Submit(ctx, "after-migration", application()); err != nil {
		t.Fatal(err)
	}
}
