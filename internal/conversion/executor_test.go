package conversion

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

type gatewayStub struct {
	create func(context.Context, CreateCommand) (string, error)
	lookup func(context.Context, string) (LookupResult, error)
	valid  func(string) bool
}

func (g gatewayStub) Create(ctx context.Context, c CreateCommand) (string, error) {
	return g.create(ctx, c)
}
func (g gatewayStub) Lookup(ctx context.Context, id string) (LookupResult, error) {
	return g.lookup(ctx, id)
}
func (g gatewayStub) ValidateLinkURL(link string) bool { return g.valid(link) }

func testExecutor(db Repository, gateway LinkGateway) Executor {
	return Executor{
		Repository: db,
		Eligibility: eligibilityFunc(func(context.Context, string, string) (preview.ChannelPosition, error) {
			return preview.ChannelPosition{AccountID: "account", ExternalPositionID: "external-position"}, nil
		}),
		Previews: preview.NewRepository(db.DB),
		Requoter: requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) {
			return preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1000,
				PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1",
				ExpiresAt: time.Now().Add(time.Minute)}, nil
		}),
		Gateway: gateway,
	}
}

func TestExecutorCreatesExactlyOnceThenReturnsSuccess(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	created, err := repo.Reserve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	gateway := gatewayStub{
		create: func(_ context.Context, c CreateCommand) (string, error) {
			calls++
			if c.RequestID != created.ID || c.TrackingID != created.TrackingID || c.ExternalProductID != "sku" || c.ExternalPositionID != "external-position" {
				t.Fatal(c)
			}
			return "https://channel.example/item", nil
		},
		lookup: func(context.Context, string) (LookupResult, error) {
			t.Fatal("unexpected lookup")
			return LookupResult{}, nil
		},
		valid: func(link string) bool { return link == "https://channel.example/item" },
	}
	executor := testExecutor(repo, gateway)
	result, err := executor.RunPending(context.Background(), created.ID)
	if err != nil || result.Status != "SUCCEEDED" || result.LinkURL == "" || result.AttemptCount != 1 || calls != 1 {
		t.Fatal(result, err, calls)
	}
	if _, err := executor.RunPending(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) || calls != 1 {
		t.Fatal(err, calls)
	}
}

func TestExecutorUncertainResultIsQueriedNeverResent(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	created, err := repo.Reserve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	creates, lookups := 0, 0
	gateway := gatewayStub{
		create: func(context.Context, CreateCommand) (string, error) { creates++; return "", context.DeadlineExceeded },
		lookup: func(_ context.Context, id string) (LookupResult, error) {
			lookups++
			if id != created.ID {
				t.Fatal(id)
			}
			if lookups == 1 {
				return LookupResult{Status: LookupUnknown}, nil
			}
			return LookupResult{Status: LookupSucceeded, LinkURL: "https://channel.example/item"}, nil
		},
		valid: func(link string) bool { return link == "https://channel.example/item" },
	}
	executor := testExecutor(repo, gateway)
	uncertain, err := executor.RunPending(context.Background(), created.ID)
	if err != nil || uncertain.Status != "FAILED_RETRYABLE" || uncertain.FailureCode != "QUERY_REQUIRED" || creates != 1 {
		t.Fatal(uncertain, err, creates)
	}
	executor.Requoter = nil // Recovery needs only the channel's result-query capability.
	still, err := executor.Recover(context.Background(), created.ID)
	if err != nil || still.Status != "FAILED_RETRYABLE" || creates != 1 {
		t.Fatal(still, err, creates)
	}
	succeeded, err := executor.Recover(context.Background(), created.ID)
	if err != nil || succeeded.Status != "SUCCEEDED" || creates != 1 || lookups != 2 {
		t.Fatal(succeeded, err, creates, lookups)
	}
	if _, err := executor.Recover(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
}

func TestExecutorRejectsChangedQuoteBeforeChannelCall(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	created, err := repo.Reserve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	executor := testExecutor(repo, gatewayStub{
		create: func(context.Context, CreateCommand) (string, error) { t.Fatal("channel called"); return "", nil },
		lookup: func(context.Context, string) (LookupResult, error) { return LookupResult{}, nil },
		valid:  func(string) bool { return true },
	})
	executor.Requoter = requoterFunc(func(context.Context, string, preview.ChannelPosition) (preview.Quote, error) {
		return preview.Quote{ExternalProductID: "sku", ProductName: "product", CouponPriceMinor: 1001,
			PromoterEstimateMinor: 100, ConsumerCashbackEstimateMinor: 50, RuleVersion: "v1", ExpiresAt: time.Now().Add(time.Minute)}, nil
	})
	result, err := executor.RunPending(context.Background(), created.ID)
	if err != nil || result.Status != "FAILED_FINAL" || result.FailureCode != "INVALID_RESULT" || result.AttemptCount != 0 {
		t.Fatal(result, err)
	}
	if _, err := executor.RunPending(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
}

func TestExecutorNoProviderFailsClosed(t *testing.T) {
	if _, err := (Executor{}).RunPending(context.Background(), "anything"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := (Executor{}).Recover(context.Background(), "anything"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestExecutorDefinitiveRejectionIsFinal(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	created, err := repo.Reserve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	executor := testExecutor(repo, gatewayStub{
		create: func(context.Context, CreateCommand) (string, error) { return "", ErrChannelRejected },
		lookup: func(context.Context, string) (LookupResult, error) {
			t.Fatal("unexpected lookup")
			return LookupResult{}, nil
		},
		valid: func(string) bool { return false },
	})
	result, err := executor.RunPending(context.Background(), created.ID)
	if err != nil || result.Status != "FAILED_FINAL" || result.FailureCode != "CHANNEL_REJECTED" || result.LinkURL != "" {
		t.Fatal(result, err)
	}
	if _, err := executor.Recover(context.Background(), created.ID); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
}

func TestExecutorConcurrentRunOnlyOneChannelCall(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	created, err := repo.Reserve(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := 0
	executor := testExecutor(repo, gatewayStub{
		create: func(context.Context, CreateCommand) (string, error) {
			mu.Lock()
			calls++
			mu.Unlock()
			return "https://channel.example/item", nil
		},
		lookup: func(context.Context, string) (LookupResult, error) { return LookupResult{}, nil },
		valid:  func(string) bool { return true },
	})
	const n = 12
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = executor.RunPending(context.Background(), created.ID) }(i)
	}
	wg.Wait()
	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatal(gotCalls)
	}
	for _, err := range errs {
		if err != nil && !errors.Is(err, ErrStateConflict) {
			t.Fatal(err)
		}
	}
}
