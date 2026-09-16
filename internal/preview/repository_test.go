package preview

import (
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
)

func sample() Snapshot {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return Snapshot{
		ID: "pv-1", OwnerUserID: "u1", PositionID: "p1", IdempotencyKey: "request-1",
		RequestFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Scene:              "home", Channel: "JD", ExternalProductID: "sku-1", ProductName: "商品",
		Currency: "CNY", CouponPriceMinor: 1234, PromoterEstimateMinor: 100,
		ConsumerCashbackEstimateMinor: 50, RuleVersion: "rule-1", EvidenceRef: "quote-1",
		UpdatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
}

func TestSnapshotValidation(t *testing.T) {
	s := sample()
	if !s.valid() {
		t.Fatal("valid snapshot rejected")
	}
	for name, change := range map[string]func(*Snapshot){
		"empty evidence":      func(s *Snapshot) { s.EvidenceRef = "" },
		"bad fingerprint":     func(s *Snapshot) { s.RequestFingerprint = "abc" },
		"negative estimate":   func(s *Snapshot) { s.PromoterEstimateMinor = -1 },
		"expired at quote":    func(s *Snapshot) { s.ExpiresAt = s.UpdatedAt },
		"excess lifetime":     func(s *Snapshot) { s.ExpiresAt = s.UpdatedAt.Add(25 * time.Hour) },
		"wrong currency":      func(s *Snapshot) { s.Currency = "USD" },
		"unsupported channel": func(s *Snapshot) { s.Channel = "OTHER" },
		"whitespace":          func(s *Snapshot) { s.PositionID = " p1 " },
	} {
		t.Run(name, func(t *testing.T) {
			bad := s
			change(&bad)
			if bad.valid() {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}

func previewDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("preview_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	for _, name := range []string{"000002_promoter_applications", "000004_promotion_positions", "000005_channel_positions", "000007_promotion_previews"} {
		b, err := os.ReadFile("../../migrations/" + name + ".up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range []string{"u1", "u2"} {
		if _, err := db.Exec(`INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED')`, user); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []struct{ id, owner string }{{"p1", "u1"}, {"p2", "u2"}} {
		if _, err := db.Exec(`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES($1,$2,'main','home','ENABLED',1)`, p.id, p.owner); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRepositoryConcurrentReplayConflictAndOwnership(t *testing.T) {
	db := previewDB(t)
	repo := NewRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const n = 12
	results := make([]Snapshot, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := sample()
			s.ID = fmt.Sprintf("pv-%d", i)
			results[i], errs[i] = repo.Save(ctx, s)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i].ID != results[0].ID {
			t.Fatalf("retry %d: %+v %v", i, results[i], errs[i])
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM promotion_previews`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	changed := sample()
	changed.RequestFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := repo.Save(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(ctx, "u2", results[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if got, err := repo.FindByID(ctx, "u1", results[0].ID); err != nil || got.CouponPriceMinor != 1234 {
		t.Fatal(got, err)
	}
	other := sample()
	other.ID, other.OwnerUserID, other.PositionID = "pv-u2", "u2", "p2"
	if _, err := repo.Save(ctx, other); err != nil {
		t.Fatal(err)
	}
	badOwner := sample()
	badOwner.ID, badOwner.IdempotencyKey, badOwner.PositionID = "pv-bad", "bad-owner", "p2"
	if _, err := repo.Save(ctx, badOwner); err == nil {
		t.Fatal("cross-owner position accepted")
	}
	if err := db.QueryRow(`SELECT count(*) FROM promotion_previews`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
