package material

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Catches raw Record exposure, wrong identity lookup and missing public fields.
func TestCatalogDetailSafeProjection(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	r := fixture()
	r.StartsAt, r.SourceUpdatedAt = now, now
	r.Region, r.Business = Region{Mode: "CITIES", CityCodes: []string{"310100"}}, "food"
	insertCatalogMaterial(t, db, 1, r)
	got, err := (Repository{DB: db}).Get(stdcontext.Background(), q.OwnerID, q.Scope, key, catalogID(1), now)
	if err != nil || got.Item == nil || !got.Capability.Allowed || got.Capability.Reason != "READY" || !got.Availability.Available || got.Availability.Reason != "AVAILABLE" {
		t.Fatalf("detail: %+v %v", got, err)
	}
	card := got.Item
	if card.ID != "00000000-0000-4000-8000-000000000001" || card.Platform != "JD" || card.Type != "PRODUCT" || card.Title != "测试商品" || card.RuleVersion != "v1" || card.Business != "food" || card.Region.Mode != "CITIES" || !reflect.DeepEqual(card.Region.CityCodes, []string{"310100"}) || !reflect.DeepEqual(card.Terminals, []string{"H5"}) || card.StartsAt == nil || !card.StartsAt.Equal(now) || !card.SourceUpdatedAt.Equal(now) || !card.EndsAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("wrong projection: %+v", card)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]json.RawMessage
	if err := json.Unmarshal(raw, &public); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(public["item"], &fields); err != nil {
		t.Fatal(err)
	}
	if len(public) != 3 || len(fields) != 11 {
		t.Fatalf("unexpected fields: %s", raw)
	}
	for _, field := range []string{"id", "platform", "type", "title", "startsAt", "endsAt", "sourceUpdatedAt", "ruleVersion", "region", "business", "terminals"} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing %s: %s", field, raw)
		}
	}
	for _, secret := range []string{"https://", "source-1", "private-proof", "media-proof", "call-proof", "canGenerate", "price", "estimate"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("leaked %s: %s", secret, raw)
		}
	}
	var tracks, requests int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM tracking_records),(SELECT count(*) FROM promotion_conversion_requests)`).Scan(&tracks, &requests); err != nil || tracks != 0 || requests != 0 {
		t.Fatal(tracks, requests, err)
	}
}

// Catches omitted CardFor validation or unsafe card retention after denial.
func TestCatalogDetailMaterialDenials(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	for n, tc := range []struct {
		name, reason string
		change       func(*Record)
	}{
		{"draft", "NOT_ACTIVE", func(r *Record) { r.Status = "DRAFT" }},
		{"suspended", "NOT_ACTIVE", func(r *Record) { r.Status = "SUSPENDED" }},
		{"removed", "NOT_ACTIVE", func(r *Record) { r.Status = "REMOVED" }},
		{"future start", "NOT_STARTED", func(r *Record) { r.StartsAt = now.Add(time.Second) }},
		{"expiry boundary", "EXPIRED", func(r *Record) { r.EndsAt = now }},
		{"future source", "SOURCE_NOT_CURRENT", func(r *Record) { r.SourceUpdatedAt = now.Add(time.Second) }},
		{"city", "REGION_MISMATCH", func(r *Record) { r.Region = Region{Mode: "CITIES", CityCodes: []string{"110100"}} }},
		{"business", "BUSINESS_MISMATCH", func(r *Record) { r.Business = "travel" }},
		{"terminal", "TERMINAL_MISMATCH", func(r *Record) { r.Terminals = []string{"WX_MINI"} }},
		{"invalid URL", "INVALID_MATERIAL", func(r *Record) { r.CanonicalURL = "https://" }},
		{"invalid proof", "INVALID_MATERIAL", func(r *Record) { r.EvidenceRef = "\u00a0private-proof" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixture()
			tc.change(&r)
			insertCatalogMaterial(t, db, n+1, r)
			got, err := (Repository{DB: db}).Get(stdcontext.Background(), q.OwnerID, q.Scope, key, catalogID(n+1), now)
			if err != nil || got.Item != nil || !got.Capability.Allowed || got.Capability.Reason != "READY" || got.Availability.Available || got.Availability.Reason != tc.reason {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	for n, change := range []func(*Record){func(r *Record) { r.Platform = "TB" }, func(r *Record) { r.Type = "ACTIVITY"; r.StartsAt = now }} {
		r := fixture()
		change(&r)
		insertCatalogMaterial(t, db, n+30, r)
	}
	for _, id := range []string{catalogID(30), catalogID(31), catalogID(99)} {
		got, err := (Repository{DB: db}).Get(stdcontext.Background(), q.OwnerID, q.Scope, key, id, now)
		if err != nil || got.Item != nil || !got.Capability.Allowed || got.Availability.Available || got.Availability.Reason != "MATERIAL_UNAVAILABLE" {
			t.Fatal(got, err)
		}
	}
}

// Catches material reads before capability gating, revoked grants and cross-owner access.
func TestCatalogDetailCapabilityGate(t *testing.T) {
	db, q, key := catalogDB(t)
	insertCatalogMaterial(t, db, 1, fixture())
	repo := Repository{DB: db}
	check := func(owner, reason string, at time.Time) {
		t.Helper()
		got, err := repo.Get(stdcontext.Background(), owner, q.Scope, key, catalogID(1), at)
		if err != nil || got.Item != nil || got.Capability.Allowed || got.Capability.Reason != reason || got.Availability.Available || got.Availability.Reason != "CAPABILITY_UNAVAILABLE" {
			t.Fatal(got, err)
		}
	}
	check(q.OwnerID, "UNCONFIGURED", now)
	readyCatalog(t, db)
	check("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "POSITION_UNAVAILABLE", now)
	check(q.OwnerID, "VERIFICATION_EXPIRED", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='DISABLED' WHERE application_id='catalog-app'`); err != nil {
		t.Fatal(err)
	}
	check(q.OwnerID, "NOT_ENABLED", now)
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='ENABLED' WHERE application_id='catalog-app'; UPDATE promotion_positions SET status='DISABLED'`); err != nil {
		t.Fatal(err)
	}
	check(q.OwnerID, "POSITION_UNAVAILABLE", now)
	if _, err := db.Exec(`UPDATE promotion_positions SET status='ENABLED'; UPDATE channel_capabilities SET status='SUSPENDED'; DROP TABLE promotion_materials`); err != nil {
		t.Fatal(err)
	}
	check(q.OwnerID, "SUSPENDED", now)
}

// Catches treating activities as product SKUs or reusing another platform's grant.
func TestCatalogDetailTypedActivities(t *testing.T) {
	db, q, key := catalogDB(t)
	for i, platform := range []string{"JD", "TB", "MT"} {
		scope := q.Scope
		scope.Platform, scope.Type, scope.Terminal = platform, "ACTIVITY", "WX_MINI"
		requested := key
		requested.Platform, requested.MaterialType, requested.Terminal = platform, "ACTIVITY", "WX_MINI"
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
		r.Platform, r.Type, r.Title, r.StartsAt, r.Terminals = platform, "ACTIVITY", "无商品价活动", now, []string{"WX_MINI"}
		insertCatalogMaterial(t, db, i+1, r)
		got, err := (Repository{DB: db}).Get(stdcontext.Background(), q.OwnerID, scope, requested, catalogID(i+1), now)
		if err != nil || !got.Capability.Allowed || !got.Availability.Available || got.Item == nil || got.Item.Platform != platform || got.Item.Type != "ACTIVITY" || got.Item.Title != "无商品价活动" || got.Item.StartsAt == nil || !got.Item.StartsAt.Equal(now) {
			t.Fatal(got, err)
		}
	}
}

// Catches a gate and metadata read from different snapshots and stale new reads.
func TestCatalogDetailReadSnapshot(t *testing.T) {
	for _, tc := range []struct{ name, sql, capability, availability string }{
		{"member", `UPDATE promoter_profiles SET status='DISABLED' WHERE application_id='catalog-app'`, "NOT_ENABLED", "CAPABILITY_UNAVAILABLE"},
		{"title", `UPDATE promotion_materials SET title='更新后的商品'`, "READY", "AVAILABLE"},
		{"expired", `UPDATE promotion_materials SET ends_at='2026-09-19T00:00:00Z'`, "READY", "EXPIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, q, key := catalogDB(t)
			readyCatalog(t, db)
			insertCatalogMaterial(t, db, 1, fixture())
			ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 20*time.Second)
			defer cancel()
			writer, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback()
			if _, err := writer.ExecContext(ctx, `LOCK TABLE promotion_materials IN ACCESS EXCLUSIVE MODE`); err != nil {
				t.Fatal(err)
			}
			var pid int
			if err := writer.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			type result struct {
				detail Detail
				err    error
			}
			read := make(chan result, 1)
			go func() {
				got, err := (Repository{DB: db}).Get(ctx, q.OwnerID, q.Scope, key, catalogID(1), now)
				read <- result{got, err}
			}()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE NOT granted AND relation='promotion_materials'::regclass AND $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case early := <-read:
					t.Fatalf("read completed before observed lock: %+v", early)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-ticker.C:
				}
			}
			if _, err := writer.ExecContext(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if err := writer.Commit(); err != nil {
				t.Fatal(err)
			}
			select {
			case old := <-read:
				if old.err != nil || old.detail.Item == nil || old.detail.Item.Title != "测试商品" || !old.detail.Capability.Allowed || old.detail.Capability.Reason != "READY" || !old.detail.Availability.Available {
					t.Fatal("mixed snapshot", old)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			fresh, err := (Repository{DB: db}).Get(ctx, q.OwnerID, q.Scope, key, catalogID(1), now)
			if err != nil || fresh.Capability.Reason != tc.capability || fresh.Capability.Allowed != (tc.capability == "READY") || fresh.Availability.Reason != tc.availability || fresh.Availability.Available != (tc.availability == "AVAILABLE") {
				t.Fatal(fresh, err)
			}
			if tc.name == "title" {
				if fresh.Item == nil || fresh.Item.Title != "更新后的商品" {
					t.Fatal(fresh)
				}
			} else if fresh.Item != nil {
				t.Fatal("denied detail retained item", fresh)
			}
		})
	}
}

// Catches accepting malformed IDs, mismatched keys or leaking storage errors/partial data.
func TestCatalogDetailInvalidAndStorageFailures(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	repo := Repository{DB: db}
	for _, id := range []string{"", "00000000-0000-0000-0000-000000000000", "00000000-0000-4000-8000-00000000000A", " 00000000-0000-4000-8000-000000000001", "1' OR true--"} {
		got, err := repo.Get(stdcontext.Background(), q.OwnerID, q.Scope, key, id, now)
		if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(got, Detail{}) {
			t.Fatal(got, err)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*Query, *time.Time)
	}{
		{"owner", func(q *Query, _ *time.Time) { q.OwnerID = "not-an-owner" }},
		{"scope", func(q *Query, _ *time.Time) { q.Scope.CityCode = "110100" }},
		{"terminal", func(q *Query, _ *time.Time) { q.Scope.Terminal = "WX_MINI" }},
		{"zero time", func(_ *Query, at *time.Time) { *at = time.Time{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy, at := q, now
			tc.change(&copy, &at)
			got, err := repo.Get(stdcontext.Background(), copy.OwnerID, copy.Scope, key, catalogID(1), at)
			if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(got, Detail{}) {
				t.Fatal(got, err)
			}
		})
	}
	wrong := key
	wrong.Kind = "PRODUCT_LINK"
	got, err := repo.Get(stdcontext.Background(), q.OwnerID, q.Scope, wrong, catalogID(1), now)
	if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(got, Detail{}) {
		t.Fatal(got, err)
	}
	got, err = (Repository{}).Get(stdcontext.Background(), q.OwnerID, q.Scope, key, catalogID(1), now)
	if !errors.Is(err, ErrUnavailable) || !reflect.DeepEqual(got, Detail{}) {
		t.Fatal(got, err)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	got, err = repo.Get(ctx, q.OwnerID, q.Scope, key, catalogID(1), now)
	if !errors.Is(err, stdcontext.Canceled) || !reflect.DeepEqual(got, Detail{}) {
		t.Fatal(got, err)
	}
	if _, err := db.Exec(`DROP TABLE promotion_materials`); err != nil {
		t.Fatal(err)
	}
	got, err = repo.Get(stdcontext.Background(), q.OwnerID, q.Scope, key, catalogID(1), now)
	if err != ErrUnavailable || !reflect.DeepEqual(got, Detail{}) {
		t.Fatal(got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = repo.Get(stdcontext.Background(), q.OwnerID, q.Scope, key, catalogID(1), now)
	if err != ErrUnavailable || !reflect.DeepEqual(got, Detail{}) {
		t.Fatal(got, err)
	}
}
