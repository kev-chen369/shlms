package conversion

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClaimWaitsForConcurrentEligibilityDisable(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{"membership", `UPDATE promoter_profiles SET status='DISABLED' WHERE user_id='u1'`},
		{"position", `UPDATE promotion_positions SET status='DISABLED' WHERE id='pos-u1'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := conversionDB(t)
			repo := Repository{DB: db}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			created, err := repo.Reserve(ctx, request())
			if err != nil {
				t.Fatal(err)
			}
			disable, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer disable.Rollback()
			var pid int
			if err := disable.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			if _, err := disable.ExecContext(ctx, tc.query); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() {
				_, err := repo.Claim(ctx, created.ID, created.Version, time.Minute)
				result <- err
			}()
			waitForDatabaseBlock(t, ctx, db, pid, result)
			if err := disable.Commit(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if !errors.Is(err, ErrStateConflict) {
					t.Fatalf("claim after disable: %v, want ErrStateConflict", err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			got, err := repo.FindByID(ctx, "u1", created.ID)
			if err != nil || got.Status != "PENDING" || got.Version != 1 || got.AttemptCount != 0 || got.LeaseExpiresAt != nil || got.ChannelRequestID != "" {
				t.Fatalf("rejected claim changed state: %+v %v", got, err)
			}
		})
	}
}

func TestClaimLeaseStartsAfterRequestLockWait(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	created, err := repo.Reserve(ctx, request())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	var pid int
	if err := lock.QueryRowContext(ctx, `SELECT pg_backend_pid() FROM promotion_conversion_requests WHERE id=$1 FOR UPDATE`, created.ID).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	var claimed Record
	const lease = 500 * time.Millisecond
	go func() {
		var err error
		claimed, err = repo.Claim(ctx, created.ID, created.Version, lease)
		result <- err
	}()
	waitForDatabaseBlock(t, ctx, db, pid, result)
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	releasedAt := time.Now()
	if err := lock.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if claimed.LeaseExpiresAt == nil || claimed.LeaseExpiresAt.Before(releasedAt.Add(400*time.Millisecond)) {
		t.Fatalf("lock wait consumed lease: released=%v expires=%v", releasedAt, claimed.LeaseExpiresAt)
	}
}

func TestClaimUpdateFailureRollsBackAndReleasesEligibilityLocks(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	created, err := repo.Reserve(ctx, request())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE FUNCTION reject_claim() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected claim failure'; END; $$;
		CREATE TRIGGER reject_claim BEFORE UPDATE ON promotion_conversion_requests FOR EACH ROW EXECUTE FUNCTION reject_claim()`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Claim(ctx, created.ID, created.Version, time.Minute); err == nil {
		t.Fatal("injected update failure accepted")
	}
	got, err := repo.FindByID(ctx, "u1", created.ID)
	if err != nil || got.Status != "PENDING" || got.Version != 1 || got.AttemptCount != 0 || got.LeaseExpiresAt != nil {
		t.Fatalf("failed claim changed state: %+v %v", got, err)
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER reject_claim ON promotion_conversion_requests;
		UPDATE promoter_profiles SET version=version+1 WHERE user_id='u1';
		UPDATE promotion_positions SET version=version+1 WHERE id='pos-u1'`); err != nil {
		t.Fatalf("eligibility locks not released: %v", err)
	}
	got, err = repo.Claim(ctx, created.ID, created.Version, time.Minute)
	if err != nil || got.Status != "PROCESSING" || got.AttemptCount != 1 || got.Version != 2 {
		t.Fatal(got, err)
	}
}
