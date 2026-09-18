package material

import (
	stdcontext "context"
	"database/sql"
	"testing"
	"time"
)

type catalogReadResult struct {
	page Page
	err  error
}

func waitCatalogBlocked(t *testing.T, ctx stdcontext.Context, db *sql.DB, blocker int, result <-chan catalogReadResult) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE NOT granted AND relation='promotion_materials'::regclass AND $1=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case early := <-result:
			t.Fatalf("reader completed before observed material lock: %+v %v", early.page, early.err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestCatalogReadSnapshotAcrossCommittedChanges(t *testing.T) {
	for _, tc := range []struct {
		name, change, reason, title string
		empty                       bool
	}{
		{"capability suspended", `UPDATE channel_capabilities SET status='SUSPENDED'`, "SUSPENDED", "", true},
		{"membership disabled", `UPDATE promoter_profiles SET status='DISABLED' WHERE application_id='catalog-app'`, "NOT_ENABLED", "", true},
		{"position disabled", `UPDATE promotion_positions SET status='DISABLED'`, "POSITION_UNAVAILABLE", "", true},
		{"new future evidence", `INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('f1111111-1111-4111-8111-111111111111','c1111111-1111-4111-8111-111111111111','operator','media-proof','source-proof','v2','call-proof','2026-09-19T00:00:01Z','2026-09-20T00:00:00Z','test-import'); UPDATE channel_capabilities SET evidence_id='f1111111-1111-4111-8111-111111111111'`, "INVALID_EVIDENCE", "", true},
		{"material suspended", `UPDATE promotion_materials SET status='SUSPENDED'`, "READY", "", true},
		{"material city changed", `UPDATE promotion_materials SET region_mode='CITIES',city_codes=ARRAY['110100']`, "READY", "", true},
		{"material business changed", `UPDATE promotion_materials SET business='hotel'`, "READY", "", true},
		{"material terminal changed", `UPDATE promotion_materials SET terminals=ARRAY['WX_MINI']`, "READY", "", true},
		{"material expired", `UPDATE promotion_materials SET ends_at='2026-09-19T00:00:00Z'`, "READY", "", true},
		{"material title changed", `UPDATE promotion_materials SET title='更新后的商品'`, "READY", "更新后的商品", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, q, key := catalogDB(t)
			readyCatalog(t, db)
			q.Limit = 1
			for n := 1; n <= 3; n++ {
				r := fixture()
				if n == 2 {
					r.EvidenceRef = "\u00a0private-proof"
				}
				insertCatalogMaterial(t, db, n, r)
			}
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
			result := make(chan catalogReadResult, 1)
			go func() {
				page, err := (Repository{DB: db}).List(ctx, q, key, now)
				result <- catalogReadResult{page, err}
			}()
			waitCatalogBlocked(t, ctx, db, pid, result)
			if _, err := writer.ExecContext(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			if err := writer.Commit(); err != nil {
				t.Fatal(err)
			}
			var old catalogReadResult
			select {
			case old = <-result:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if old.err != nil || !old.page.Capability.Allowed || old.page.Capability.Reason != "READY" || len(old.page.Items) != 1 || old.page.Items[0].ID != catalogID(1) || old.page.Items[0].Title != "测试商品" || old.page.NextCursor == "" {
				t.Fatal("mixed read snapshot", old.page, old.err)
			}
			fresh, err := (Repository{DB: db}).List(ctx, q, key, now)
			if err != nil || fresh.Items == nil || fresh.Capability.Reason != tc.reason || fresh.Capability.Allowed != (tc.reason == "READY") {
				t.Fatal("new request missed committed change", fresh, err)
			}
			if tc.empty {
				if len(fresh.Items) != 0 || fresh.NextCursor != "" {
					t.Fatal(fresh)
				}
			} else {
				if len(fresh.Items) != 1 || fresh.Items[0].Title != tc.title || fresh.NextCursor == "" {
					t.Fatal(fresh)
				}
			}
			// The old cursor is a position, not a cached authorization/snapshot.
			continued := q
			continued.Cursor = old.page.NextCursor
			next, err := (Repository{DB: db}).List(ctx, continued, key, now)
			if err != nil || next.Capability != fresh.Capability || next.Items == nil || next.NextCursor != "" {
				t.Fatal("cursor reused stale grant", next, err)
			}
			if tc.empty {
				if len(next.Items) != 0 {
					t.Fatal(next)
				}
			} else {
				if len(next.Items) != 1 || next.Items[0].ID != catalogID(3) || next.Items[0].Title != tc.title {
					t.Fatal(next)
				}
			}
			for _, statement := range []string{`SELECT count(*) FROM tracking_records`, `SELECT count(*) FROM promotion_conversion_requests`} {
				var count int
				if err := db.QueryRowContext(ctx, statement).Scan(&count); err != nil || count != 0 {
					t.Fatal(count, err)
				}
			}
			var count int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM promotion_materials`).Scan(&count); err != nil || count != 3 {
				t.Fatal("metadata lost", count, err)
			}
			wantProofs := 1
			if tc.name == "new future evidence" {
				wantProofs = 2
			}
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM channel_capability_evidence`).Scan(&count); err != nil || count != wantProofs {
				t.Fatal("proof history lost", count, err)
			}
		})
	}
}
