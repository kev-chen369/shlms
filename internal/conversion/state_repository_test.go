package conversion

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func TestClaimConcurrentOnceAndRecoverUncertainResult(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	created, err := repo.Reserve(ctx, request())
	if err != nil {
		t.Fatal(err)
	}
	const n = 12
	results := make([]Record, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.Claim(ctx, created.ID, created.Version, time.Minute)
		}(i)
	}
	wg.Wait()
	successes := 0
	var claimed Record
	for i := range results {
		if errs[i] == nil {
			successes++
			claimed = results[i]
		} else if !errors.Is(errs[i], ErrStateConflict) {
			t.Fatal(errs[i])
		}
	}
	if successes != 1 || claimed.Status != "PROCESSING" || claimed.Version != 2 || claimed.AttemptCount != 1 || claimed.ChannelRequestID != created.ID || claimed.LeaseExpiresAt == nil {
		t.Fatal(successes, claimed)
	}
	if _, err := repo.MarkSucceeded(ctx, claimed.ID, claimed.ChannelRequestID, claimed.Version, "http://unsafe.example"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := repo.MarkSucceeded(ctx, claimed.ID, claimed.ChannelRequestID, claimed.Version, "https://127.0.0.1/item"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	uncertain, err := repo.MarkUncertain(ctx, claimed.ID, claimed.ChannelRequestID, claimed.Version)
	if err != nil || uncertain.Status != "FAILED_RETRYABLE" || uncertain.FailureCode != "QUERY_REQUIRED" || uncertain.LeaseExpiresAt != nil || uncertain.Version != 3 {
		t.Fatal(uncertain, err)
	}
	candidates, err := repo.RecoveryCandidates(ctx, 10)
	if err != nil || len(candidates) != 1 || candidates[0].ID != created.ID {
		t.Fatal(candidates, err)
	}
	if _, err := repo.Claim(ctx, uncertain.ID, uncertain.Version, time.Minute); !errors.Is(err, ErrStateConflict) {
		t.Fatal("uncertain request resent", err)
	}
	succeeded, err := repo.MarkSucceeded(ctx, uncertain.ID, uncertain.ChannelRequestID, uncertain.Version, "https://channel.example/item")
	if err != nil || succeeded.Status != "SUCCEEDED" || succeeded.LinkURL != "https://channel.example/item" || succeeded.FailureCode != "" || succeeded.Version != 4 {
		t.Fatal(succeeded, err)
	}
	if _, err := repo.MarkFinal(ctx, succeeded.ID, succeeded.ChannelRequestID, succeeded.Version, "CHANNEL_REJECTED"); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
	candidates, err = repo.RecoveryCandidates(ctx, 10)
	if err != nil || len(candidates) != 0 {
		t.Fatal(candidates, err)
	}
}

func TestConversionStateMigrationDownAndUp(t *testing.T) {
	db := conversionDB(t)
	for _, direction := range []string{"down", "up"} {
		b, err := os.ReadFile("../../migrations/000009_conversion_state." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatal(direction, err)
		}
	}
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='promotion_conversion_requests' AND column_name='version')`).Scan(&exists); err != nil || !exists {
		t.Fatal(exists, err)
	}
}

func TestStateFinalFailureAndExpiredLease(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx := context.Background()
	first := request()
	first.IdempotencyKey = "final"
	created, err := repo.Reserve(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.Claim(ctx, created.ID, created.Version, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.MarkFinal(ctx, claimed.ID, claimed.ChannelRequestID, claimed.Version, "raw upstream secret"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	final, err := repo.MarkFinal(ctx, claimed.ID, claimed.ChannelRequestID, claimed.Version, "CHANNEL_REJECTED")
	if err != nil || final.Status != "FAILED_FINAL" || final.FailureCode != "CHANNEL_REJECTED" || final.LinkURL != "" {
		t.Fatal(final, err)
	}
	if _, err := repo.MarkSucceeded(ctx, final.ID, final.ChannelRequestID, final.Version, "https://channel.example/item"); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
	second := request()
	second.IdempotencyKey = "expired-lease"
	created, err = repo.Reserve(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err = repo.Claim(ctx, created.ID, created.Version, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE promotion_conversion_requests SET lease_expires_at=CURRENT_TIMESTAMP - interval '1 minute' WHERE id=$1`, claimed.ID); err != nil {
		t.Fatal(err)
	}
	candidates, err := repo.RecoveryCandidates(ctx, 10)
	if err != nil || len(candidates) != 1 || candidates[0].ID != claimed.ID {
		t.Fatal(candidates, err)
	}
	if _, err := repo.Claim(ctx, claimed.ID, claimed.Version, time.Minute); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
}

func TestClaimRejectsDisabledMembership(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx := context.Background()
	created, err := repo.Reserve(ctx, request())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='DISABLED' WHERE user_id='u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Claim(ctx, created.ID, created.Version, time.Minute); !errors.Is(err, ErrStateConflict) {
		t.Fatal(err)
	}
}
