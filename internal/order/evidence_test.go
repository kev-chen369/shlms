package order

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

func orderTestDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("order_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = admin.Close() })
	if _, err := dbmigrate.Run(context.Background(), db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRawEvidenceEncryptedAndIdempotent(t *testing.T) {
	db := orderTestDB(t)
	key := bytes.Repeat([]byte{17}, 32)
	store, err := NewStore(db, "test-v1", key)
	if err != nil {
		t.Fatal(err)
	}
	input := RawEvent{Channel: "JD", EventID: "channel-event-1", ExternalOrderID: "order-1",
		EventType: "ORDER", OccurredAt: time.Now().UTC(), Payload: []byte(`{"order":"order-1","buyerPhone":"private-marker"}`)}
	const workers = 12
	results := make([]Evidence, workers)
	errs := make([]error, workers)
	created := make([]bool, workers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], created[i], errs[i] = store.Save(context.Background(), input)
		}(i)
	}
	wg.Wait()
	countCreated := 0
	for i, err := range errs {
		if err != nil || results[i].ID != results[0].ID {
			t.Fatal(i, results[i], err)
		}
		if created[i] {
			countCreated++
		}
	}
	if countCreated != 1 {
		t.Fatal("created", countCreated)
	}
	var count int
	var ciphertext []byte
	if err := db.QueryRow(`SELECT count(*) FROM order_raw_events`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow(`SELECT payload_ciphertext FROM order_raw_events WHERE id=$1`, results[0].ID).Scan(&ciphertext); err != nil || bytes.Contains(ciphertext, []byte("private-marker")) {
		t.Fatal(err)
	}
	read, err := store.Read(context.Background(), results[0].ID)
	if err != nil || !bytes.Equal(read.Payload, input.Payload) {
		t.Fatal(read, err)
	}
	changed := input
	changed.Payload = []byte(`{"order":"order-1","buyerPhone":"changed"}`)
	if _, _, err := store.Save(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed payload", err)
	}
	changed = input
	changed.ExternalOrderID = "other-order"
	if _, _, err := store.Save(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed order", err)
	}
	_, err = db.Exec(`UPDATE order_raw_events SET payload_ciphertext=set_byte(payload_ciphertext,0,get_byte(payload_ciphertext,0)#1) WHERE id=$1`, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(context.Background(), results[0].ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("tamper accepted", err)
	}
}

func TestRawEvidenceRejectsInvalidInputAndMissingKey(t *testing.T) {
	db := orderTestDB(t)
	if _, err := NewStore(db, "v1", []byte("short")); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	store, err := NewStore(db, "v1", bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	base := RawEvent{Channel: "MT", EventID: "e1", ExternalOrderID: "o1", EventType: "REFUND", OccurredAt: time.Now(), Payload: []byte(`{"refund":1}`)}
	for _, invalid := range []RawEvent{
		{Channel: "PDD", EventID: "e1", ExternalOrderID: "o1", EventType: "ORDER", OccurredAt: base.OccurredAt, Payload: base.Payload},
		{Channel: "MT", EventID: "e1", ExternalOrderID: "o1", EventType: "REFUND", OccurredAt: base.OccurredAt, Payload: []byte(`null`)},
		{Channel: "MT", EventID: "e1", ExternalOrderID: "o1", EventType: "REFUND", OccurredAt: base.OccurredAt, Payload: bytes.Repeat([]byte("x"), (64<<10)+1)},
	} {
		if _, _, err := store.Save(context.Background(), invalid); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	got, _, err := store.Save(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewStore(db, "v2", bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Read(context.Background(), got.ID); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatal(err)
	}
	if _, err := store.Read(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
