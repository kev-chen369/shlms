package material

import (
	stdcontext "context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/capability"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

const catalogOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

func catalogDB(t *testing.T) (*sql.DB, Query, capability.Key) {
	t.Helper()
	db := materialTestDB(t)
	if _, err := dbmigrate.Run(stdcontext.Background(), db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED')`,
		`INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status) VALUES('catalog-app',$1,'catalog-key','test','home','v1','2026-09-19T00:00:00Z','ENABLED')`,
		`UPDATE promoter_profiles SET status='ENABLED',application_id='catalog-app' WHERE user_id=$1`,
		`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES('catalog-position',$1,'main','home','ENABLED',1)`,
	} {
		if _, err := db.Exec(query, catalogOwner); err != nil {
			t.Fatal(err)
		}
	}
	q := Query{OwnerID: catalogOwner, Scope: Context{"JD", "PRODUCT", "310100", "food", "H5"}, Limit: 2}
	key := capability.Key{Platform: "JD", MaterialType: "PRODUCT", Kind: "CATALOG", MediaID: "catalog-media", PositionID: "catalog-position", Scene: "home", Terminal: "H5", CityCode: "310100", Business: "food"}
	if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business) VALUES('c1111111-1111-4111-8111-111111111111','JD','PRODUCT','CATALOG','catalog-media','catalog-position','home','H5','310100','food');
 INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('c2222222-2222-4222-8222-222222222222','c1111111-1111-4111-8111-111111111111','operator','media-proof','source-proof','v1','call-proof','2026-09-18T00:00:00Z','2026-09-20T00:00:00Z','test-import')`); err != nil {
		t.Fatal(err)
	}
	return db, q, key
}
func readyCatalog(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`UPDATE channel_capabilities SET status='READY',evidence_id='c2222222-2222-4222-8222-222222222222'`); err != nil {
		t.Fatal(err)
	}
}
func catalogID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func insertCatalogMaterial(t *testing.T, db *sql.DB, n int, r Record) {
	t.Helper()
	cities, _ := json.Marshal(r.Region.CityCodes)
	if string(cities) == "null" {
		cities = []byte("[]")
	}
	terminals, _ := json.Marshal(r.Terminals)
	var starts any
	if !r.StartsAt.IsZero() {
		starts = r.StartsAt
	}
	_, err := db.Exec(`INSERT INTO promotion_materials(id,platform,material_type,external_material_id,canonical_url,title,status,starts_at,ends_at,source_updated_at,rule_version,evidence_ref,region_mode,city_codes,business,terminals)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,ARRAY(SELECT jsonb_array_elements_text($14::jsonb)),$15,ARRAY(SELECT jsonb_array_elements_text($16::jsonb)))`, catalogID(n), r.Platform, r.Type, fmt.Sprintf("source-%d", n), r.CanonicalURL, r.Title, r.Status, starts, r.EndsAt, r.SourceUpdatedAt, r.RuleVersion, r.EvidenceRef, r.Region.Mode, string(cities), r.Business, string(terminals))
	if err != nil {
		t.Fatal(err)
	}
}

// Catches raw-row pagination that loses valid entries behind malformed imports.
func TestCatalogRepositoryPaginationAndSafeProjection(t *testing.T) {
	db, q, key := catalogDB(t)
	repo := Repository{DB: db}
	ctx := stdcontext.Background()
	for n := 1; n <= 10; n++ {
		r := fixture()
		switch n {
		case 1:
			r.CanonicalURL = "https://"
		case 3:
			r.EvidenceRef = "\u00a0private-proof"
		case 5:
			r.Status = "DRAFT"
		case 7:
			r.EndsAt = now
		case 8:
			r.SourceUpdatedAt = now.Add(time.Second)
		case 9:
			r.StartsAt = now.Add(time.Minute)
		case 4:
			r.StartsAt = now
			r.SourceUpdatedAt = now
		}
		insertCatalogMaterial(t, db, n, r)
	}
	page, err := repo.List(ctx, q, key, now)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" || page.Capability.Reason != "UNCONFIGURED" {
		t.Fatal(page, err)
	}
	readyCatalog(t, db)
	q.Limit = 1
	page, err = repo.List(ctx, q, key, now)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != catalogID(2) || !page.Capability.Allowed || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	q.Cursor = page.NextCursor
	q.Limit = 2
	page, err = repo.List(ctx, q, key, now)
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != catalogID(4) || page.Items[1].ID != catalogID(6) || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	if page.Items[0].StartsAt == nil || !page.Items[0].StartsAt.Equal(now) || !page.Items[0].SourceUpdatedAt.Equal(now) {
		t.Fatal(page.Items[0])
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-proof", "source-", "item.jd.com", "media-proof", "catalog-media", "canonical", "externalMaterial", "canGenerate", "estimate", "price"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("leaked %s: %s", secret, raw)
		}
	}
	q.Cursor = page.NextCursor
	page, err = repo.List(ctx, q, key, now)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != catalogID(10) || page.NextCursor != "" {
		t.Fatal(page, err)
	}
	page.Items[0].Terminals[0] = "BROKEN"
	page, err = repo.List(ctx, q, key, now)
	if err != nil || page.Items[0].Terminals[0] != "H5" {
		t.Fatal(page, err)
	}
	for _, statement := range []string{`SELECT count(*) FROM tracking_records`, `SELECT count(*) FROM promotion_conversion_requests`} {
		var count int
		if err := db.QueryRow(statement).Scan(&count); err != nil || count != 0 {
			t.Fatal(count, err)
		}
	}
}

// Catches public scope leaks and use of material wildcard rules for capability.
func TestCatalogRepositoryScopeAndRevocation(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	repo := Repository{DB: db}
	ctx := stdcontext.Background()
	insertCatalogMaterial(t, db, 100, fixture())
	for i, change := range []func(*Record){
		func(r *Record) { r.Platform = "TB" }, func(r *Record) { r.Type = "ACTIVITY"; r.StartsAt = now },
		func(r *Record) { r.Region = Region{"CITIES", []string{"110100"}} }, func(r *Record) { r.Business = "hotel" },
		func(r *Record) { r.Terminals = []string{"WX_MINI"} }, func(r *Record) { r.Status = "DRAFT" }, func(r *Record) { r.Status = "SUSPENDED" }, func(r *Record) { r.Status = "REMOVED" },
	} {
		r := fixture()
		change(&r)
		insertCatalogMaterial(t, db, i+1, r)
	}
	page, err := repo.List(ctx, q, key, now)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != catalogID(100) || page.NextCursor != "" {
		t.Fatal(page, err)
	}
	r := fixture()
	r.Region = Region{"CITIES", []string{"310100"}}
	r.Business = "food"
	insertCatalogMaterial(t, db, 101, r)
	page, err = repo.List(ctx, q, key, now)
	if err != nil || len(page.Items) != 2 || page.NextCursor != "" {
		t.Fatal(page, err)
	}
	for _, tc := range []struct{ sql, reason string }{
		{`UPDATE channel_capabilities SET status='SUSPENDED'`, "SUSPENDED"},
		{`UPDATE channel_capabilities SET status='PENDING_VERIFICATION'`, "PENDING_VERIFICATION"},
		{`UPDATE promotion_positions SET status='DISABLED'`, "POSITION_UNAVAILABLE"},
	} {
		if _, err := db.Exec(tc.sql); err != nil {
			t.Fatal(err)
		}
		page, err = repo.List(ctx, q, key, now)
		if err != nil || len(page.Items) != 0 || page.Items == nil || page.NextCursor != "" || page.Capability.Reason != tc.reason {
			t.Fatal(page, err)
		}
	}
	if _, err := db.Exec(`UPDATE promotion_positions SET status='ENABLED'; UPDATE promoter_profiles SET status='DISABLED' WHERE application_id='catalog-app'`); err != nil {
		t.Fatal(err)
	}
	page, err = repo.List(ctx, q, key, now)
	if err != nil || len(page.Items) != 0 || page.Capability.Reason != "NOT_ENABLED" {
		t.Fatal(page, err)
	}
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='ENABLED' WHERE application_id='catalog-app'`); err != nil {
		t.Fatal(err)
	}
	readyCatalog(t, db)
	page, err = repo.List(ctx, q, key, now.Add(24*time.Hour))
	if err != nil || len(page.Items) != 0 || page.Capability.Reason != "VERIFICATION_EXPIRED" {
		t.Fatal(page, err)
	}
	other := q
	other.OwnerID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	page, err = repo.List(ctx, other, key, now)
	if err != nil || len(page.Items) != 0 || page.Capability.Reason != "POSITION_UNAVAILABLE" {
		t.Fatal(page, err)
	}
	other = q
	other.Scope.CityCode = ""
	otherKey := key
	otherKey.CityCode = ""
	page, err = repo.List(ctx, other, otherKey, now)
	if err != nil || len(page.Items) != 0 || page.Capability.Reason != "UNCONFIGURED" {
		t.Fatal(page, err)
	}
}

func TestCatalogRepositoryInvalidRequestsAndStorage(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	repo := Repository{DB: db}
	ctx := stdcontext.Background()
	cursor, err := EncodeCursor(q, catalogID(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Query, *capability.Key){
		func(q *Query, k *capability.Key) { q.OwnerID = "" }, func(q *Query, k *capability.Key) { q.Limit = 101 },
		func(q *Query, k *capability.Key) { q.Cursor = "bad" }, func(q *Query, k *capability.Key) {
			q.Cursor = cursor
			q.OwnerID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		},
		func(q *Query, k *capability.Key) { q.Cursor = cursor; q.Scope.Business = "hotel"; k.Business = "hotel" },
		func(q *Query, k *capability.Key) { k.Platform = "TB" }, func(q *Query, k *capability.Key) { k.Kind = "PRODUCT_LINK" },
		func(q *Query, k *capability.Key) { k.MediaID = "\u00a0media" }, func(q *Query, k *capability.Key) { k.PositionID = "" }, func(q *Query, k *capability.Key) { k.Scene = strings.Repeat("s", 81) },
	} {
		requested := q
		requestedKey := key
		change(&requested, &requestedKey)
		page, err := repo.List(ctx, requested, requestedKey, now)
		if !errors.Is(err, ErrInvalid) || page.Items != nil || page.NextCursor != "" || page.Capability != (capability.Decision{}) {
			t.Fatal(page, err)
		}
	}
	page, err := repo.List(ctx, q, key, time.Time{})
	if !errors.Is(err, ErrInvalid) || page.Items != nil {
		t.Fatal(page, err)
	}
	page, err = (Repository{}).List(ctx, q, key, now)
	if !errors.Is(err, ErrUnavailable) || page.Items != nil {
		t.Fatal(page, err)
	}
	canceled, cancel := stdcontext.WithCancel(ctx)
	cancel()
	page, err = repo.List(canceled, q, key, now)
	if !errors.Is(err, stdcontext.Canceled) || page.Items != nil {
		t.Fatal(page, err)
	}
	if _, err := db.Exec(`DROP TABLE promotion_materials`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE channel_capabilities SET status='SUSPENDED'`); err != nil {
		t.Fatal(err)
	}
	page, err = repo.List(ctx, q, key, now)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.Capability.Reason != "SUSPENDED" {
		t.Fatal("denied grant must not query absent material table", page, err)
	}
	readyCatalog(t, db)
	page, err = repo.List(ctx, q, key, now)
	if !errors.Is(err, ErrUnavailable) || page.Items != nil || strings.Contains(err.Error(), "promotion_materials") {
		t.Fatal(page, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	page, err = repo.List(ctx, q, key, now)
	if !errors.Is(err, ErrUnavailable) || page.Items != nil {
		t.Fatal(page, err)
	}
}

func TestCatalogRepositoryTypedActivities(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	insertCatalogMaterial(t, db, 1, fixture())
	for i, platform := range []string{"JD", "TB", "MT"} {
		requested := q
		requested.Scope.Platform = platform
		requested.Scope.Type = "ACTIVITY"
		requested.Scope.Terminal = "WX_MINI"
		requestedKey := key
		requestedKey.Platform = platform
		requestedKey.MaterialType = "ACTIVITY"
		requestedKey.Terminal = "WX_MINI"
		declaration := fmt.Sprintf("d0000000-0000-4000-8000-%012d", i+1)
		proof := fmt.Sprintf("e0000000-0000-4000-8000-%012d", i+1)
		if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business) VALUES($1,$2,'ACTIVITY','CATALOG','catalog-media','catalog-position','home','WX_MINI','310100','food')`, declaration, platform); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES($1,$2,'operator','media-proof','source-proof','v1','call-proof','2026-09-18T00:00:00Z','2026-09-20T00:00:00Z','test-import')`, proof, declaration); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE channel_capabilities SET status='READY',evidence_id=$1 WHERE id=$2`, proof, declaration); err != nil {
			t.Fatal(err)
		}
		r := fixture()
		r.Platform = platform
		r.Type = "ACTIVITY"
		r.CanonicalURL = "https://example.com/authorized-activity"
		r.Title = "无商品价活动"
		r.StartsAt = now
		r.Region = Region{"CITIES", []string{"310100"}}
		r.Business = "food"
		r.Terminals = []string{"WX_MINI"}
		insertCatalogMaterial(t, db, i+2, r)
		page, err := (Repository{DB: db}).List(stdcontext.Background(), requested, requestedKey, now)
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != catalogID(i+2) || page.Items[0].Platform != platform || page.Items[0].Type != "ACTIVITY" || page.Items[0].StartsAt == nil || !page.Items[0].StartsAt.Equal(now) || page.NextCursor != "" {
			t.Fatal(page, err)
		}
	}
}

func TestCatalogRepositoryDiscardsPartialPageOnLaterBatchError(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	q.Limit = 1
	for n := 1; n <= 3; n++ {
		r := fixture()
		if n == 2 {
			r.EvidenceRef = "\u00a0invalid-proof"
		}
		insertCatalogMaterial(t, db, n, r)
	}
	// Inject an actual SQL failure only in a later batch. Keep actual imported
	// rows and all other columns; no repository/mock behavior is substituted.
	if _, err := db.Exec(`ALTER TABLE promotion_materials RENAME TO material_backing;
 CREATE FUNCTION material_test_title(material_id uuid, original_title text) RETURNS text LANGUAGE plpgsql STABLE AS $$
 BEGIN
 IF material_id='00000000-0000-4000-8000-000000000003'::uuid THEN RAISE EXCEPTION 'private injected database error'; END IF;
 RETURN original_title;
 END; $$;
 CREATE VIEW promotion_materials AS SELECT id,platform,material_type,external_material_id,canonical_url,material_test_title(id,title) AS title,status,starts_at,ends_at,source_updated_at,rule_version,evidence_ref,region_mode,city_codes,business,terminals FROM material_backing`); err != nil {
		t.Fatal(err)
	}
	// The read-only function is STABLE: a VOLATILE view projection prevents
	// flattening and evaluates the failing later ID below sort/limit (confirmed
	// with EXPLAIN). Verify the prefix succeeds instead of assuming evaluation.
	rows, err := db.Query(`SELECT title FROM promotion_materials ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatal(err)
		}
		count++
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil || count != 2 {
		t.Fatal("first batch must succeed", rowErr, closeErr, count)
	}
	page, err := (Repository{DB: db}).List(stdcontext.Background(), q, key, now)
	if !errors.Is(err, ErrUnavailable) || page.Items != nil || page.NextCursor != "" || page.Capability != (capability.Decision{}) || strings.Contains(err.Error(), "private") {
		t.Fatal("partial page or raw SQL failure leaked", page, err)
	}
}
