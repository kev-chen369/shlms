package tracking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresTrackingMigrationAndConcurrentIdempotency(t *testing.T) {
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	schema := fmt.Sprintf("tracking_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000001_tracking_records.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply tracking migration: %v", err)
	}

	repository := NewPostgresRepository(db)
	var sequence atomic.Int64
	service := NewService(repository, func() string {
		return fmt.Sprintf("TRK-%d", sequence.Add(1))
	}, func() time.Time { return time.Now().UTC() })
	input := CreateInput{
		IdempotencyKey: "request-concurrent", UserID: "user-1",
		Channel: ChannelJD, ExternalProductID: "sku-1", Source: "product_detail",
	}
	const callers = 12
	records := make([]Record, callers)
	errorsByCaller := make([]error, callers)
	var wg sync.WaitGroup
	for i := range records {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			records[index], errorsByCaller[index] = service.Create(ctx, input)
		}(i)
	}
	wg.Wait()
	for i := range records {
		if errorsByCaller[i] != nil || records[i].ID != records[0].ID {
			t.Fatalf("call %d: record=%#v error=%v; first=%#v", i, records[i], errorsByCaller[i], records[0])
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tracking_records WHERE idempotency_key = $1", input.IdempotencyKey).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stored records = %d, want 1", count)
	}

	different := input
	different.UserID = "user-2"
	if _, err := service.Create(ctx, different); !errors.Is(err, ErrConflict) {
		t.Fatalf("reused key for another user: error=%v, want ErrConflict", err)
	}
	if _, err := repository.FindByIdempotencyKey(ctx, "not-found"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: error=%v, want ErrNotFound", err)
	}
}
