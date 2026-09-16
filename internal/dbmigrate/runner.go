// Package dbmigrate applies immutable PostgreSQL up migrations in order.
package dbmigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var migrationName = regexp.MustCompile(`^(\d{6})_[a-z0-9_]+\.up\.sql$`)

type migration struct{ version, name, checksum, sql string }
type Result struct {
	Applied        []string
	AlreadyApplied []string
}

func readMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	items := []migration{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version := matches[1]
		if seen[version] {
			return nil, fmt.Errorf("duplicate migration version %s", version)
		}
		seen[version] = true
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			return nil, fmt.Errorf("empty migration %s", entry.Name())
		}
		checksum := sha256.Sum256(b)
		items = append(items, migration{version: version, name: entry.Name(), checksum: hex.EncodeToString(checksum[:]), sql: string(b)})
	}
	if len(items) == 0 {
		return nil, errors.New("no up migrations found")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	return items, nil
}

// Run uses one dedicated connection and an advisory lock for the entire
// migration sequence. Each version and its ledger row commit atomically.
func Run(ctx context.Context, db *sql.DB, dir string) (Result, error) {
	items, err := readMigrations(dir)
	if err != nil {
		return Result{}, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return Result{}, err
	}
	defer conn.Close()
	// Fixed namespaced key: all instances of this service serialize migrations.
	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock(8420260916)`); err != nil {
		return Result{}, err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(8420260916)`)
	}()
	if _, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return Result{}, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT version,checksum FROM schema_migrations`)
	if err != nil {
		return Result{}, err
	}
	applied := map[string]string{}
	for rows.Next() {
		var version, checksum string
		if err = rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return Result{}, err
		}
		applied[version] = checksum
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Result{}, err
	}
	rows.Close()
	known := map[string]bool{}
	for _, item := range items {
		known[item.version] = true
		if old, ok := applied[item.version]; ok && old != item.checksum {
			return Result{}, fmt.Errorf("migration %s checksum differs from applied version", item.version)
		}
	}
	for version := range applied {
		if !known[version] {
			return Result{}, fmt.Errorf("applied migration %s is missing from directory", version)
		}
	}
	latestApplied := ""
	for version := range applied {
		if version > latestApplied {
			latestApplied = version
		}
	}
	result := Result{Applied: []string{}, AlreadyApplied: []string{}}
	for _, item := range items {
		if _, ok := applied[item.version]; ok {
			result.AlreadyApplied = append(result.AlreadyApplied, item.name)
			continue
		}
		if item.version < latestApplied {
			return result, fmt.Errorf("unapplied migration %s precedes version %s already applied", item.version, latestApplied)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		if _, err = tx.ExecContext(ctx, item.sql); err != nil {
			_ = tx.Rollback()
			return result, fmt.Errorf("apply %s: %w", item.name, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, item.version, item.checksum); err != nil {
			_ = tx.Rollback()
			return result, err
		}
		if err = tx.Commit(); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, item.name)
	}
	return result, nil
}
