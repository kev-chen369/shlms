package material

import (
	stdcontext "context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

func materialTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN is not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*cfg)
	schema := fmt.Sprintf("material_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	return db
}

func materialMigration(t *testing.T, db *sql.DB, direction string) {
	t.Helper()
	raw, err := os.ReadFile("../../migrations/000022_promotion_materials." + direction + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestMaterialSchemaConstraintsAndDown(t *testing.T) {
	db := materialTestDB(t)
	materialMigration(t, db, "up")
	_, err := db.Exec(`INSERT INTO promotion_materials(id,platform,material_type,external_material_id,canonical_url,title,ends_at,source_updated_at,rule_version,evidence_ref)
 VALUES('11111111-1111-4111-8111-111111111111','JD','PRODUCT','123','https://item.jd.com/123.html','product','2026-09-20T00:00:00Z','2026-09-19T00:00:00Z','v1','proof')`)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM promotion_materials`).Scan(&status); err != nil || status != "DRAFT" {
		t.Fatalf("unapproved default: %s %v", status, err)
	}
	_, err = db.Exec(`INSERT INTO promotion_materials(id,platform,material_type,external_material_id,canonical_url,title,starts_at,ends_at,source_updated_at,rule_version,evidence_ref,region_mode,city_codes,terminals)
 VALUES('22222222-2222-4222-8222-222222222222','MT','ACTIVITY','123','https://example.com/activity','price-free activity','2026-09-19T00:00:00Z','2026-09-20T00:00:00Z','2026-09-19T00:00:00Z','v1','proof','CITIES',ARRAY['310100'],ARRAY['WX_MINI'])`)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(`UPDATE promotion_materials SET region_mode='CITIES',city_codes=ARRAY(SELECT x::text FROM generate_series(1,63) AS x)||ARRAY[repeat('a',32)] WHERE platform='JD'`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("valid maximum city scope rejected: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, expression string }{
		{"nil UUID", `id='00000000-0000-0000-0000-000000000000'`},
		{"unknown platform", `platform='PDD'`}, {"MT product", `platform='MT'`},
		{"activity missing start", `material_type='ACTIVITY'`}, {"unknown status", `status='READY'`},
		{"empty source id", `external_material_id=''`}, {"title bytes", `title=repeat('中',86)`},
		{"empty proof", `evidence_ref=''`}, {"non HTTPS", `canonical_url='http://example.com'`},
		{"inverted window", `starts_at=ends_at`}, {"infinite end", `ends_at='infinity'`},
		{"out of JSON range", `source_updated_at='10000-01-01T00:00:00Z'`},
		{"end upper bound", `ends_at='10000-01-01T00:00:00Z'`},
		{"source lower bound", `source_updated_at='0001-01-01 00:00:00 BC'`},
		{"infinite start", `starts_at='infinity'`},
		{"nationwide city", `city_codes=ARRAY['310100']`}, {"city scope empty", `region_mode='CITIES'`},
		{"duplicate cities", `region_mode='CITIES',city_codes=ARRAY['310100','310100']`},
		{"null city", `region_mode='CITIES',city_codes=ARRAY[NULL::text]`},
		{"too many cities", `region_mode='CITIES',city_codes=ARRAY(SELECT x::text FROM generate_series(1,65) AS x)`},
		{"city bytes", `region_mode='CITIES',city_codes=ARRAY[repeat('a',33)]`},
		{"unknown region", `region_mode='UNKNOWN'`},
		{"empty terminals", `terminals=ARRAY[]::text[]`}, {"duplicate terminals", `terminals=ARRAY['H5','H5']`},
		{"unknown terminal", `terminals=ARRAY['APP']`}, {"multidimensional", `terminals=ARRAY[['H5']]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			_, err = tx.Exec(`UPDATE promotion_materials SET ` + tc.expression + ` WHERE platform='JD'`)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("expected CHECK violation, got %v", err)
			}
		})
	}
	_, err = db.Exec(`INSERT INTO promotion_materials SELECT '33333333-3333-4333-8333-333333333333'::uuid,platform,material_type,external_material_id,canonical_url,title,status,starts_at,ends_at,source_updated_at,rule_version,evidence_ref,region_mode,city_codes,business,terminals FROM promotion_materials WHERE platform='JD'`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate identity accepted: %v", err)
	}
	_, err = db.Exec(`INSERT INTO promotion_materials(id,platform,material_type,external_material_id,canonical_url,title,starts_at,ends_at,source_updated_at,rule_version,evidence_ref)
 VALUES('44444444-4444-4444-8444-444444444444','JD','ACTIVITY','123','https://example.com/activity','separate activity','2026-09-19T00:00:00Z','2026-09-20T00:00:00Z','2026-09-19T00:00:00Z','v1','proof')`)
	if err != nil {
		t.Fatalf("same external id in different type rejected: %v", err)
	}
	materialMigration(t, db, "down")
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('promotion_materials') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("down table remains: %v %v", exists, err)
	}
	if err := db.QueryRow(`SELECT to_regprocedure('promotion_material_array_valid(text[],integer,integer,integer,text[])') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("down helper remains: %v %v", exists, err)
	}
}
