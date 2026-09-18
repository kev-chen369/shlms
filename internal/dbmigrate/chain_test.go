package dbmigrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestActualMigrationChainUpDownUp(t *testing.T) {
	db := isolatedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := "../../migrations"
	items, err := readMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(ctx, db, dir)
	if err != nil || len(result.Applied) != len(items) {
		t.Fatal(result, err)
	}
	if err := Verify(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		raw, err := os.ReadFile(filepath.Join(dir, strings.TrimSuffix(item.name, ".up.sql")+".down.sql"))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Only the synthetic schema's ledger is changed. Production Run does
		// not expose a down/reset command or alter historical checksums.
		if _, err := tx.ExecContext(ctx, string(raw)); err != nil {
			tx.Rollback()
			t.Fatalf("down %s: %v", item.version, err)
		}
		deleted, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=$1`, item.version)
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		count, err := deleted.RowsAffected()
		if err != nil || count != 1 {
			tx.Rollback()
			t.Fatal(item.version, count, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("ledger residue", count, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relkind IN ('r','p','v','m','S') AND relname<>'schema_migrations'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("business relation residue", count, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_proc WHERE pronamespace=current_schema()::regnamespace`).Scan(&count); err != nil || count != 0 {
		t.Fatal("function residue", count, err)
	}
	result, err = Run(ctx, db, dir)
	if err != nil || len(result.Applied) != len(items) {
		t.Fatal("reapply", result, err)
	}
	if err := Verify(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	result, err = Run(ctx, db, dir)
	if err != nil || len(result.Applied) != 0 || len(result.AlreadyApplied) != len(items) {
		t.Fatal("repeat", result, err)
	}
}
