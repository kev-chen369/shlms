package conversion

import (
	"context"
	"errors"
	"testing"
)

func TestExecutorPersistsResultAfterCallerCancellation(t *testing.T) {
	for _, success := range []bool{false, true} {
		name := "uncertain"
		if success {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			repo := Repository{DB: conversionDB(t)}
			created, err := repo.Reserve(context.Background(), request())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			creates, lookups := 0, 0
			executor := testExecutor(repo, gatewayStub{
				create: func(callCtx context.Context, c CreateCommand) (string, error) {
					creates++
					if c.RequestID != created.ID {
						t.Fatal(c)
					}
					cancel()
					if !errors.Is(callCtx.Err(), context.Canceled) {
						t.Fatal("gateway context was not canceled")
					}
					if success {
						return "https://channel.example/item", nil
					}
					return "", callCtx.Err()
				},
				lookup: func(queryCtx context.Context, id string) (LookupResult, error) {
					lookups++
					if id != created.ID {
						t.Fatal(id)
					}
					// A verified result may arrive as the recovery caller disconnects.
					cancel()
					if !errors.Is(queryCtx.Err(), context.Canceled) {
						t.Fatal("lookup context was not canceled")
					}
					return LookupResult{Status: LookupSucceeded, LinkURL: "https://channel.example/item"}, nil
				},
				valid: func(link string) bool { return link == "https://channel.example/item" },
			})
			got, err := executor.RunPending(ctx, created.ID)
			if err != nil {
				t.Fatalf("persist after canceled Create: %v", err)
			}
			if !success {
				if got.Status != "FAILED_RETRYABLE" || got.FailureCode != "QUERY_REQUIRED" || got.LinkURL != "" || got.LeaseExpiresAt != nil {
					t.Fatalf("uncertain result lost: %+v", got)
				}
				if _, err := executor.RunPending(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) {
					t.Fatalf("uncertain work was runnable again: %v", err)
				}
				ctx, cancel = context.WithCancel(context.Background())
				defer cancel()
				got, err = executor.Recover(ctx, created.ID)
				if err != nil || lookups != 1 {
					t.Fatalf("persist after canceled Lookup: %+v %v lookups=%d", got, err, lookups)
				}
			}
			stored, err := repo.FindByID(context.Background(), "u1", created.ID)
			if err != nil || stored.Status != "SUCCEEDED" || stored.LinkURL != "https://channel.example/item" || stored.ChannelRequestID != created.ID || stored.AttemptCount != 1 || creates != 1 || got.ID != stored.ID {
				t.Fatalf("durable result: %+v %v creates=%d", stored, err, creates)
			}
			if _, err := executor.RunPending(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("terminal result created again: %v", err)
			}
			if _, err := executor.Recover(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("terminal result queried again: %v", err)
			}
			wantLookups := 1
			if success {
				wantLookups = 0
			}
			final, err := repo.FindByID(context.Background(), "u1", created.ID)
			if err != nil || final.Status != stored.Status || final.Version != stored.Version || final.AttemptCount != 1 || final.LinkURL != stored.LinkURL || creates != 1 || lookups != wantLookups {
				t.Fatalf("terminal guard changed result: %+v %v creates=%d lookups=%d", final, err, creates, lookups)
			}
		})
	}
}
