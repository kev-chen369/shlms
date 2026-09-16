package conversion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

func TestExecutorRejectsExpiryBeforeChannelCreate(t *testing.T) {
	for _, phase := range []string{"requote-preview", "claim-preview", "claim-quote"} {
		t.Run(phase, func(t *testing.T) {
			db := conversionDB(t)
			repo := Repository{DB: db}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			created, err := repo.Reserve(ctx, request())
			if err != nil {
				t.Fatal(err)
			}
			expiresAt := time.Now().Add(3 * time.Second)
			if phase != "claim-quote" {
				if _, err := db.ExecContext(ctx, `UPDATE promotion_previews SET expires_at=$1 WHERE id='pv-u1'`, expiresAt); err != nil {
					t.Fatal(err)
				}
			}
			waitUntilExpired := func() {
				timer := time.NewTimer(time.Until(expiresAt) + 20*time.Millisecond)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			creates, requotes := 0, 0
			executor := testExecutor(repo, gatewayStub{
				create: func(context.Context, CreateCommand) (string, error) {
					creates++
					return "https://channel.example/item", nil
				},
				lookup: func(context.Context, string) (LookupResult, error) {
					t.Error("pre-call rejection must not query channel")
					return LookupResult{}, nil
				},
				valid: func(string) bool { return true },
			})
			executor.Requoter = requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) {
				requotes++
				if !expiresAt.After(time.Now()) {
					t.Error("fixture expired before requote; wait path not exercised")
				}
				if phase == "requote-preview" {
					waitUntilExpired()
				}
				quoteExpiresAt := time.Now().Add(time.Minute)
				if phase == "claim-quote" {
					quoteExpiresAt = expiresAt
				}
				return preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000,
					PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: quoteExpiresAt}, nil
			})
			var got Record
			if phase == "requote-preview" {
				got, err = executor.RunPending(ctx, created.ID)
			} else {
				lock, lockErr := db.BeginTx(ctx, nil)
				if lockErr != nil {
					t.Fatal(lockErr)
				}
				defer lock.Rollback()
				var pid int
				if err := lock.QueryRowContext(ctx, `SELECT pg_backend_pid() FROM promotion_conversion_requests WHERE id=$1 FOR UPDATE`, created.ID).Scan(&pid); err != nil {
					t.Fatal(err)
				}
				result := make(chan error, 1)
				go func() {
					var runErr error
					got, runErr = executor.RunPending(ctx, created.ID)
					result <- runErr
				}()
				waitForDatabaseBlock(t, ctx, db, pid, result)
				waitUntilExpired()
				if err := lock.Commit(); err != nil {
					t.Fatal(err)
				}
				select {
				case err = <-result:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			if err != nil || got.Status != "FAILED_FINAL" || got.FailureCode != "INVALID_RESULT" || got.LinkURL != "" || got.LeaseExpiresAt != nil || creates != 0 || requotes != 1 {
				t.Fatalf("expired work sent to channel: %+v %v creates=%d requotes=%d", got, err, creates, requotes)
			}
			wantAttempts, wantVersion := 1, int64(3)
			if phase == "requote-preview" {
				wantAttempts, wantVersion = 0, 2
			}
			if got.AttemptCount != wantAttempts || got.Version != wantVersion {
				t.Fatalf("rejection transition: attempts=%d version=%d", got.AttemptCount, got.Version)
			}
			stored, err := repo.FindByID(ctx, "u1", created.ID)
			if err != nil || stored.Status != "FAILED_FINAL" || stored.Version != got.Version || stored.LinkURL != "" {
				t.Fatal(stored, err)
			}
			if _, err := executor.RunPending(ctx, created.ID); !errors.Is(err, ErrStateConflict) {
				t.Fatal("rejected work runnable again", err)
			}
			if _, err := executor.Recover(ctx, created.ID); !errors.Is(err, ErrStateConflict) {
				t.Fatal("pre-call rejection recoverable", err)
			}
			if creates != 0 || requotes != 1 {
				t.Fatalf("terminal rejection called channel: creates=%d requotes=%d", creates, requotes)
			}
		})
	}
}
