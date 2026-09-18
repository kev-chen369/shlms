package capability

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

func capabilitySchemaDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("capability_test_%d", time.Now().UnixNano())
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
	// Copy actual prerequisite SQL, not a fake schema; isolate migration 23's down.
	dir := t.TempDir()
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") && entry.Name() < "000023" {
			raw, err := os.ReadFile(filepath.Join("../../migrations", entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, entry.Name()), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := dbmigrate.Run(context.Background(), db, dir); err != nil {
		t.Fatal(err)
	}
	capabilitySQL(t, db, "up")
	if _, err := db.Exec(`INSERT INTO promoter_profiles(user_id,status) VALUES('owner','NOT_APPLIED'); INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES('p1','owner','main','home','ENABLED',1)`); err != nil {
		t.Fatal(err)
	}
	return db
}
func capabilitySQL(t *testing.T, db *sql.DB, direction string) {
	t.Helper()
	raw, err := os.ReadFile("../../migrations/000023_channel_capabilities." + direction + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(raw)); err != nil {
		t.Fatal(err)
	}
}
func capabilityReject(t *testing.T, db *sql.DB, code, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

const capabilityInsert = `INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

func TestCapabilitySchemaScopesAndEvidence(t *testing.T) {
	db := capabilitySchemaDB(t)
	base := []any{"11111111-1111-4111-8111-111111111111", "JD", "PRODUCT", "CATALOG", "media1", "p1", "home", "H5", "", "", "UNCONFIGURED"}
	if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, base[:10]...); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM channel_capabilities`).Scan(&status); err != nil || status != "UNCONFIGURED" {
		t.Fatalf("default closed: %s %v", status, err)
	}
	cases := []struct {
		name  string
		index int
		value any
		code  string
	}{
		{"nil id", 0, "00000000-0000-0000-0000-000000000000", "23514"}, {"platform", 1, "PDD", "23514"}, {"MT product", 1, "MT", "23514"}, {"type", 2, "COUPON", "23514"},
		{"kind", 3, "UNKNOWN", "23514"}, {"activity kind on product", 3, "ACTIVITY_LINK", "23514"}, {"media", 4, "", "23514"}, {"media bytes", 4, strings.Repeat("a", 129), "23514"},
		{"missing position", 5, "missing", "23503"}, {"empty scene", 6, "", "23514"}, {"terminal", 7, "APP", "23514"}, {"city bytes", 8, strings.Repeat("c", 33), "23514"}, {"business bytes", 9, strings.Repeat("b", 41), "23514"},
		{"unknown status", 10, "ACTIVE", "23514"}, {"ready without proof", 10, "READY", "23514"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]any(nil), base...)
			args[0] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
			args[6] = fmt.Sprintf("bad-%d", i)
			args[tc.index] = tc.value
			capabilityReject(t, db, tc.code, capabilityInsert, args...)
		})
	}
	duplicate := append([]any(nil), base...)
	duplicate[0] = "33333333-3333-4333-8333-333333333333"
	capabilityReject(t, db, "23505", capabilityInsert, duplicate...)
	second := append([]any(nil), base...)
	second[0] = "44444444-4444-4444-8444-444444444444"
	second[1] = "MT"
	second[2] = "ACTIVITY"
	second[3] = "ACTIVITY_LINK"
	if _, err := db.Exec(capabilityInsert, second...); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('22222222-2222-4222-8222-222222222222',$1,'operator','media-proof','source-proof','v1','call-proof','2026-09-19T00:00:00Z','2026-09-20T00:00:00Z','audited-import')`, base[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE channel_capabilities SET status='READY',evidence_id='22222222-2222-4222-8222-222222222222' WHERE id=$1`, base[0]); err != nil {
		t.Fatal(err)
	}
	capabilityReject(t, db, "23503", `UPDATE channel_capabilities SET status='READY',evidence_id='22222222-2222-4222-8222-222222222222' WHERE id=$1`, second[0])
	if _, err := db.Exec(`INSERT INTO promoter_profiles(user_id,status) VALUES('other-owner','NOT_APPLIED'); INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES('p2','other-owner','other','home','ENABLED',1)`); err != nil {
		t.Fatal(err)
	}
	capabilityReject(t, db, "23514", `UPDATE channel_capabilities SET position_id='p2' WHERE id=$1`, base[0])
	capabilityReject(t, db, "23514", `UPDATE channel_capabilities SET city_code='310100' WHERE id=$1`, base[0])
	for _, expression := range []string{`platform='TB'`, `material_type='ACTIVITY'`, `kind='PRODUCT_LINK'`, `media_id='media2'`, `scene='other'`, `terminal='WX_MINI'`, `business='food'`} {
		capabilityReject(t, db, "23514", `UPDATE channel_capabilities SET `+expression+` WHERE id=$1`, base[0])
	}
	capabilityReject(t, db, "55000", `UPDATE channel_capability_evidence SET real_call_evidence_ref='replacement'`)
	capabilityReject(t, db, "55000", `DELETE FROM channel_capability_evidence`)
	capabilityReject(t, db, "55000", `TRUNCATE channel_capability_evidence CASCADE`)
	if _, err := db.Exec(`UPDATE channel_capabilities SET status='SUSPENDED' WHERE id=$1`, base[0]); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM channel_capability_evidence`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("proof lost: %d %v", count, err)
	}
	capabilitySQL(t, db, "down")
	for _, object := range []string{"channel_capabilities", "channel_capability_evidence"} {
		var exists bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, object).Scan(&exists); err != nil || exists {
			t.Fatalf("down left %s: %v %v", object, exists, err)
		}
	}
	for _, function := range []string{"prevent_channel_capability_key_mutation()", "prevent_channel_capability_evidence_mutation()", "channel_capability_text_valid(text,integer,boolean)"} {
		var exists bool
		if err := db.QueryRow(`SELECT to_regprocedure($1) IS NOT NULL`, function).Scan(&exists); err != nil || exists {
			t.Fatalf("down left %s: %v %v", function, exists, err)
		}
	}
	if err := db.QueryRow(`SELECT count(*) FROM promotion_positions WHERE id='p1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("old position lost: %d %v", count, err)
	}
}

func TestCapabilityEvidenceCompletenessAndWindow(t *testing.T) {
	db := capabilitySchemaDB(t)
	if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal) VALUES('11111111-1111-4111-8111-111111111111','JD','PRODUCT','CATALOG','media1','p1','home','H5');
 INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111','operator','media-proof','source-proof','v1','call-proof','2026-09-19T00:00:00Z','2026-09-20T00:00:00Z','audited-import')`); err != nil {
		t.Fatal(err)
	}
	columns := []string{`'33333333-3333-4333-8333-333333333333'::uuid`, "capability_id", "owner_id", "media_approval_ref", "source_approval_ref", "interface_version", "real_call_evidence_ref", "verified_at", "expires_at", "recorded_by"}
	for _, tc := range []struct {
		name             string
		index            int
		expression, code string
	}{
		{"nil proof UUID", 0, `'00000000-0000-0000-0000-000000000000'::uuid`, "23514"},
		{"missing declaration", 1, `'44444444-4444-4444-8444-444444444444'::uuid`, "23503"},
		{"owner", 2, `''`, "23514"}, {"media", 3, `''`, "23514"}, {"source", 4, `''`, "23514"}, {"interface", 5, `''`, "23514"}, {"real call", 6, `''`, "23514"}, {"audit actor", 9, `''`, "23514"},
		{"proof bytes", 6, `repeat('a',129)`, "23514"}, {"proof control", 4, `E'bad\nproof'`, "23514"},
		{"zero verified", 7, `'0001-01-01T00:00:00Z'::timestamptz`, "23514"}, {"year range", 8, `'10000-01-01T00:00:00Z'::timestamptz`, "23514"},
		{"infinite verified", 7, `'-infinity'::timestamptz`, "23514"}, {"infinite expiry", 8, `'infinity'::timestamptz`, "23514"}, {"equal window", 8, `verified_at`, "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			values := append([]string(nil), columns...)
			values[tc.index] = tc.expression
			_, err = tx.Exec(`INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) SELECT ` + strings.Join(values, ",") + ` FROM channel_capability_evidence`)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("want SQLSTATE %s, got %v", tc.code, err)
			}
		})
	}
}
