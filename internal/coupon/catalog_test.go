package coupon

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

func testDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("coupon_test_%d", time.Now().UnixNano())
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

func TestCatalogOnlyListsVerifiedCurrentMaterial(t *testing.T) {
	db := testDB(t)
	catalog := Catalog{DB: db}
	page, err := catalog.List(context.Background(), ListInput{Limit: 20})
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	insert := `INSERT INTO coupon_catalog(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
        VALUES($1,$2,'PLATFORM_CLAIM','真实渠道活动','PRODUCT',100,1000,'v1','review-1',now(),now()-interval '1 hour',$3,$4)`
	for _, row := range []struct {
		id, platform string
		expiry       time.Time
		enabled      bool
	}{
		{"a", "JD", time.Now().Add(time.Hour), true},
		{"b", "JD", time.Now().Add(time.Hour), true},
		{"c", "TB", time.Now().Add(time.Hour), true},
		{"d", "JD", time.Now().Add(-time.Minute), true},
		{"e", "JD", time.Now().Add(time.Hour), false},
	} {
		if _, err := db.Exec(insert, row.id, row.platform, row.expiry, row.enabled); err != nil {
			t.Fatal(err)
		}
	}
	first, err := catalog.List(context.Background(), ListInput{Platform: "JD", Limit: 1})
	if err != nil || len(first.Items) != 1 || first.Items[0].ID != "a" || first.Items[0].ActionLabel != "前往平台领券" || first.NextCursor == "" {
		t.Fatal(first, err)
	}
	second, err := catalog.List(context.Background(), ListInput{Platform: "JD", Limit: 1, Cursor: first.NextCursor})
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != "b" || second.NextCursor != "" {
		t.Fatal(second, err)
	}
	if _, err := catalog.List(context.Background(), ListInput{Platform: "TB", Limit: 1, Cursor: first.NextCursor}); err != ErrInvalid {
		t.Fatal("cross-platform cursor accepted", err)
	}
}
