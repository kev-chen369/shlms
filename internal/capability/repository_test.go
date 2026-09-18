package capability

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const repositoryOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

// Uses actual migration constraints, not mocked eligibility or declarations.
func TestRepositoryExactScopeAndEligibility(t *testing.T) {
	db := capabilitySchemaDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	key := Key{"JD", "PRODUCT", "CATALOG", "media1", "p1", "home", "H5", "", ""}
	repo := Repository{DB: db}
	// Seed a separate real owner, application and position, then enable profile.
	for _, query := range []string{
		`INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED')`,
		`INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status) VALUES('repo-app',$1,'repo-key','test','home','v1','2026-09-19T00:00:00Z','ENABLED')`,
		`UPDATE promoter_profiles SET status='ENABLED',application_id='repo-app' WHERE user_id=$1`,
		`UPDATE promotion_positions SET owner_user_id=$1 WHERE id='p1'`,
	} {
		if _, err := db.Exec(query, repositoryOwner); err != nil {
			t.Fatal(err)
		}
	}
	check := func(owner string, requested Key, at time.Time, allowed bool, reason string) {
		t.Helper()
		got, err := repo.Check(ctx, owner, requested, at)
		if err != nil || got.Allowed != allowed || got.Reason != reason {
			t.Fatalf("%+v %v, want %v/%s", got, err, allowed, reason)
		}
	}
	check(repositoryOwner, key, now, false, "UNCONFIGURED")
	if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal) VALUES('11111111-1111-4111-8111-111111111111','JD','PRODUCT','CATALOG','media1','p1','home','H5');
 INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111','audited-responsible-person','media-proof','source-proof','v1','call-proof','2026-09-19T00:00:00Z','2026-09-20T00:00:00Z','test-import');
 UPDATE channel_capabilities SET status='READY',evidence_id='22222222-2222-4222-8222-222222222222'`); err != nil {
		t.Fatal(err)
	}
	check(repositoryOwner, key, now, true, "READY")
	check(repositoryOwner, key, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), true, "READY")
	check(repositoryOwner, key, now.Add(-24*time.Hour), false, "INVALID_EVIDENCE")
	check(repositoryOwner, key, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), false, "VERIFICATION_EXPIRED")
	for _, tc := range []struct {
		name   string
		mutate func(*Key)
		reason string
	}{
		{"platform", func(k *Key) { k.Platform = "TB" }, "UNCONFIGURED"},
		{"type", func(k *Key) { k.MaterialType = "ACTIVITY" }, "UNCONFIGURED"},
		{"kind", func(k *Key) { k.Kind = "PRODUCT_LINK" }, "UNCONFIGURED"},
		{"media", func(k *Key) { k.MediaID = "other" }, "UNCONFIGURED"},
		{"position", func(k *Key) { k.PositionID = "missing" }, "POSITION_UNAVAILABLE"},
		{"scene", func(k *Key) { k.Scene = "other" }, "POSITION_UNAVAILABLE"},
		{"terminal", func(k *Key) { k.Terminal = "WX_MINI" }, "UNCONFIGURED"},
		{"city", func(k *Key) { k.CityCode = "310100" }, "UNCONFIGURED"},
		{"business", func(k *Key) { k.Business = "food" }, "UNCONFIGURED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requested := key
			tc.mutate(&requested)
			check(repositoryOwner, requested, now, false, tc.reason)
		})
	}
	check("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", key, now, false, "POSITION_UNAVAILABLE")
	// A non-default scope must be found, not mistaken for an empty wildcard.
	if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business) VALUES('33333333-3333-4333-8333-333333333333','TB','ACTIVITY','ACTIVITY_LINK','media2','p1','home','WX_MINI','310100','food');
 INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('44444444-4444-4444-8444-444444444444','33333333-3333-4333-8333-333333333333','operator','media-proof','source-proof','v1','call-proof','2026-09-19T00:00:00Z','2026-09-20T00:00:00Z','test-import');
 UPDATE channel_capabilities SET status='READY',evidence_id='44444444-4444-4444-8444-444444444444' WHERE id='33333333-3333-4333-8333-333333333333'`); err != nil {
		t.Fatal(err)
	}
	activity := Key{"TB", "ACTIVITY", "ACTIVITY_LINK", "media2", "p1", "home", "WX_MINI", "310100", "food"}
	check(repositoryOwner, activity, now, true, "READY")
	for _, requested := range []Key{
		{"TB", "ACTIVITY", "ACTIVITY_LINK", "media2", "p1", "home", "WX_MINI", "", "food"},
		{"TB", "ACTIVITY", "ACTIVITY_LINK", "media2", "p1", "home", "WX_MINI", "310100", ""},
	} {
		check(repositoryOwner, requested, now, false, "UNCONFIGURED")
	}
	for _, status := range []string{"UNCONFIGURED", "PENDING_VERIFICATION", "SUSPENDED", "READY"} {
		if _, err := db.Exec(`UPDATE channel_capabilities SET status=$1`, status); err != nil {
			t.Fatal(err)
		}
		check(repositoryOwner, key, now, status == "READY", status)
	}
	// PostgreSQL's basic btrim does not reject Unicode edge whitespace. The
	// repository must still pass stored evidence through the full domain policy.
	if _, err := db.Exec(`INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by)
 SELECT '55555555-5555-4555-8555-555555555555',capability_id,owner_id,media_approval_ref,chr(160)||'source-proof',interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by FROM channel_capability_evidence WHERE id='22222222-2222-4222-8222-222222222222';
 UPDATE channel_capabilities SET evidence_id='55555555-5555-4555-8555-555555555555' WHERE id='11111111-1111-4111-8111-111111111111'`); err != nil {
		t.Fatal(err)
	}
	check(repositoryOwner, key, now, false, "INVALID_EVIDENCE")
	if _, err := db.Exec(`UPDATE channel_capabilities SET evidence_id='22222222-2222-4222-8222-222222222222' WHERE id='11111111-1111-4111-8111-111111111111'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promotion_positions SET status='DISABLED' WHERE id='p1'`); err != nil {
		t.Fatal(err)
	}
	check(repositoryOwner, key, now, false, "POSITION_UNAVAILABLE")
	if _, err := db.Exec(`UPDATE promotion_positions SET status='ENABLED' WHERE id='p1'; UPDATE promoter_profiles SET status='DISABLED' WHERE application_id='repo-app'`); err != nil {
		t.Fatal(err)
	}
	check(repositoryOwner, key, now, false, "NOT_ENABLED")
	for _, status := range []string{"NOT_APPLIED", "PENDING", "REJECTED", "DISABLED"} {
		if _, err := db.Exec(`UPDATE promoter_profiles SET status=$1,application_id=CASE WHEN $1='NOT_APPLIED' THEN NULL ELSE 'repo-app' END WHERE user_id=$2`, status, repositoryOwner); err != nil {
			t.Fatal(err)
		}
		check(repositoryOwner, key, now, false, "NOT_ENABLED")
	}
	got, _ := repo.Check(ctx, repositoryOwner, key, now)
	raw, err := json.Marshal(got)
	if err != nil || string(raw) != `{"allowed":false,"reason":"NOT_ENABLED"}` {
		t.Fatal(string(raw), err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM channel_capabilities WHERE status='READY'`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM channel_capability_evidence`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
}

func TestRepositoryFailsClosedOnInvalidInputAndStorage(t *testing.T) {
	key := Key{"JD", "PRODUCT", "CATALOG", "media1", "p1", "home", "H5", "", ""}
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	for _, owner := range []string{"", "owner", strings.ToUpper(repositoryOwner), "00000000-0000-0000-0000-000000000000", repositoryOwner + "' OR true--"} {
		got, err := (Repository{}).Check(context.Background(), owner, key, now)
		if !errors.Is(err, ErrInvalid) || got != (Decision{}) {
			t.Fatal(got, err)
		}
	}
	for _, tc := range []struct {
		k  Key
		at time.Time
	}{{Key{}, now}, {key, time.Time{}}} {
		got, err := (Repository{}).Check(context.Background(), repositoryOwner, tc.k, tc.at)
		if !errors.Is(err, ErrInvalid) || got != (Decision{}) {
			t.Fatal(got, err)
		}
	}
	got, err := (Repository{}).Check(context.Background(), repositoryOwner, key, now)
	if !errors.Is(err, ErrUnavailable) || got != (Decision{}) {
		t.Fatal(got, err)
	}
	db := capabilitySchemaDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = (Repository{DB: db}).Check(ctx, repositoryOwner, key, now)
	if !errors.Is(err, context.Canceled) || got != (Decision{}) {
		t.Fatal(got, err)
	}
	if _, err := db.Exec(`DROP TABLE channel_capabilities CASCADE`); err != nil {
		t.Fatal(err)
	}
	got, err = (Repository{DB: db}).Check(context.Background(), repositoryOwner, key, now)
	if !errors.Is(err, ErrUnavailable) || got != (Decision{}) || strings.Contains(err.Error(), "channel_capabilities") {
		t.Fatal(got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = (Repository{DB: db}).Check(context.Background(), repositoryOwner, key, now)
	if !errors.Is(err, ErrUnavailable) || got != (Decision{}) || strings.Contains(err.Error(), "sql:") {
		t.Fatal(got, err)
	}
}
