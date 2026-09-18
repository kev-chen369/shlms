package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func isolatedDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(8)
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	return db
}

func TestRunActualMigrationsConcurrentlyAndRepeatedly(t *testing.T) {
	db := isolatedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := "../../migrations"
	migrations, err := readMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := len(migrations)
	const workers = 4
	results := make([]Result, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = Run(ctx, db, dir) }(i)
	}
	wg.Wait()
	appliedTotal := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatal(i, errs[i])
		}
		appliedTotal += len(results[i].Applied)
	}
	if appliedTotal != want {
		t.Fatal("migration applied more than once", results)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != want {
		t.Fatal(count, err)
	}
	for _, table := range []string{"tracking_records", "promoter_profiles", "promoter_applications", "promotion_positions", "promotion_previews", "promotion_conversion_requests", "promotion_share_events", "channel_positions", "channel_position_config_events", "admin_principals", "admin_permissions", "promotion_materials", "channel_capabilities", "channel_capability_evidence"} {
		var exists bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil || !exists {
			t.Fatal(table, err)
		}
	}
	again, err := Run(ctx, db, dir)
	if err != nil || len(again.Applied) != 0 || len(again.AlreadyApplied) != want {
		t.Fatal(again, err)
	}
	if err := Verify(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsModifiedAndMissingAppliedMigrations(t *testing.T) {
	db := isolatedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := Run(ctx, db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		b, err := os.ReadFile(filepath.Join("../../migrations", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, entry.Name()), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(dir, "000001_tracking_records.up.sql")
	b, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(first, append(b, []byte("\n-- modified")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(ctx, db, dir); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
	if err = os.WriteFile(first, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(first); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(ctx, db, dir); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal(err)
	}
}

func TestMigrationFailureRollsBackVersion(t *testing.T) {
	db := isolatedDB(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_invalid.up.sql"), []byte("CREATE TABLE good(id int); SELECT missing FROM no_such_table;"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), db, dir); err == nil {
		t.Fatal("invalid migration succeeded")
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('good') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal(exists, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestRunRejectsLateOlderMigration(t *testing.T) {
	db := isolatedDB(t)
	dir := t.TempDir()
	second := filepath.Join(dir, "000002_second.up.sql")
	if err := os.WriteFile(second, []byte("CREATE TABLE second_table(id int);"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), db, dir); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "000001_first.up.sql")
	if err := os.WriteFile(first, []byte("CREATE TABLE first_table(id int);"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), db, dir); err == nil || !strings.Contains(err.Error(), "precedes") {
		t.Fatal(err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('first_table') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal(exists, err)
	}
}
