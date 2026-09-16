package promoter

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func channelPositionDB(t *testing.T) (PostgresRepository, string) {
	t.Helper()
	db := positionsDB(t)
	repo := NewPostgresRepository(db)
	p, err := (PositionService{Repository: repo}).ChangePosition(context.Background(), "u1", createPosition("position-1"))
	if err != nil {
		t.Fatal(err)
	}
	return repo, p.ID
}

func configureActor() AdminActor {
	return AdminActor{ID: "channel-admin", Permissions: map[string]bool{ConfigureChannelPermission: true}}
}
func channelInput(id string) ConfigureChannelPositionInput {
	return ConfigureChannelPositionInput{PositionID: id, Channel: "JD", AccountID: "account-1", ExternalPositionID: "jd-position-1", ExpectedVersion: 0, IdempotencyKey: "mapping-1"}
}

func TestConfigureChannelPositionAuthorization(t *testing.T) {
	s := ChannelPositionService{Repository: channelPositionRepoFunc(func(context.Context, string, ConfigureChannelPositionInput) (ChannelPosition, error) {
		t.Fatal("unauthorized repository access")
		return ChannelPosition{}, nil
	})}
	in := channelInput("p1")
	if _, err := s.Configure(context.Background(), AdminActor{ID: "ordinary-user"}, in); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	in.Channel = "PDD"
	if _, err := s.Configure(context.Background(), configureActor(), in); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	in = channelInput("p1")
	in.ExternalPositionID = " "
	if _, err := s.Configure(context.Background(), configureActor(), in); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
}

type channelPositionRepoFunc func(context.Context, string, ConfigureChannelPositionInput) (ChannelPosition, error)

func (f channelPositionRepoFunc) ConfigureChannelPosition(ctx context.Context, actor string, in ConfigureChannelPositionInput) (ChannelPosition, error) {
	return f(ctx, actor, in)
}

func TestPostgresChannelPositionConcurrentIdempotency(t *testing.T) {
	repo, id := channelPositionDB(t)
	s := ChannelPositionService{Repository: repo}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const callers = 12
	results := make([]ChannelPosition, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = s.Configure(ctx, configureActor(), channelInput(id))
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i].Version != 1 || results[i].Status != "PENDING_VERIFICATION" || results[i].PositionID != id || results[i].AccountID != "account-1" || results[i].ExternalPositionID != "jd-position-1" || !results[i].ConfiguredAt.Equal(results[0].ConfiguredAt) {
			t.Fatalf("caller %d: %+v %v", i, results[i], errs[i])
		}
	}
	in := channelInput(id)
	in.ExternalPositionID = "other"
	if _, err := s.Configure(ctx, configureActor(), in); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	in = channelInput(id)
	in.IdempotencyKey = "mapping-2"
	in.ExpectedVersion = 1
	in.ExternalPositionID = "jd-position-2"
	updated, err := s.Configure(ctx, configureActor(), in)
	if err != nil || updated.Version != 2 || updated.Status != "PENDING_VERIFICATION" {
		t.Fatal(updated, err)
	}
	var records, events int
	if err = repo.db.QueryRow(`SELECT count(*) FROM channel_positions`).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if err = repo.db.QueryRow(`SELECT count(*) FROM channel_position_config_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if records != 1 || events != 2 {
		t.Fatal(records, events)
	}
	if _, err = repo.db.Exec(`UPDATE channel_position_config_events SET actor_id='tampered'`); err == nil {
		t.Fatal("audit changed")
	}
	if _, err = repo.db.Exec(`DELETE FROM channel_position_config_events`); err == nil {
		t.Fatal("audit deleted")
	}
	// A failed audit write must undo the channel mapping update in the same transaction.
	if _, err = repo.db.Exec(`ALTER TABLE channel_position_config_events ADD CONSTRAINT force_event_failure CHECK (idempotency_key <> 'rollback-key')`); err != nil {
		t.Fatal(err)
	}
	rollback := channelInput(id)
	rollback.IdempotencyKey = "rollback-key"
	rollback.ExpectedVersion = 2
	rollback.ExternalPositionID = "jd-position-3"
	if _, err = s.Configure(ctx, configureActor(), rollback); err == nil {
		t.Fatal("audit failure accepted")
	}
	var storedExternal string
	var storedVersion int64
	if err = repo.db.QueryRow(`SELECT external_position_id,version FROM channel_positions WHERE position_id=$1`, id).Scan(&storedExternal, &storedVersion); err != nil || storedExternal != "jd-position-2" || storedVersion != 2 {
		t.Fatal(storedExternal, storedVersion, err)
	}
}

func TestPostgresChannelPositionOwnershipConflictAndRollback(t *testing.T) {
	repo, id := channelPositionDB(t)
	s := ChannelPositionService{Repository: repo}
	ctx := context.Background()
	first, err := s.Configure(ctx, configureActor(), channelInput(id))
	if err != nil {
		t.Fatal(err)
	}
	other, err := (PositionService{Repository: repo}).ChangePosition(ctx, "u1", createPosition("position-2"))
	if err != nil {
		t.Fatal(err)
	}
	dup := channelInput(other.ID)
	dup.IdempotencyKey = "other-key"
	if _, err = s.Configure(ctx, configureActor(), dup); !errors.Is(err, ErrExternalPositionConflict) {
		t.Fatal(err)
	}
	if _, err = repo.db.Exec(`ALTER TABLE channel_position_config_events ADD CONSTRAINT force_config_event_failure CHECK (idempotency_key <> 'rollback-key')`); err != nil {
		t.Fatal(err)
	}
	change := channelInput(id)
	change.ExpectedVersion = 1
	change.IdempotencyKey = "rollback-key"
	change.ExternalPositionID = "changed"
	if _, err = s.Configure(ctx, configureActor(), change); err == nil {
		t.Fatal("audit failure accepted")
	}
	var currentExternal string
	var version int64
	if err = repo.db.QueryRow(`SELECT external_position_id,version FROM channel_positions WHERE position_id=$1`, id).Scan(&currentExternal, &version); err != nil || currentExternal != first.ExternalPositionID || version != 1 {
		t.Fatal(currentExternal, version, err)
	}
	if _, err = (PositionService{Repository: repo}).ChangePosition(ctx, "u1", PositionCommand{Action: PositionDisable, ID: id, Version: 1, IdempotencyKey: "stop-position"}); err != nil {
		t.Fatal(err)
	}
	change.IdempotencyKey = "after-stop"
	if _, err = s.Configure(ctx, configureActor(), change); !errors.Is(err, ErrTransition) {
		t.Fatal(err)
	}
	// A disabled member cannot receive new mappings, even if a position remains enabled.
	if _, err = (AdminService{Repository: repo}).Execute(ctx, adminActor(), AdminCommand{Action: DisableAction, TargetID: "u1", Reason: "policy", Version: 2, IdempotencyKey: "stop-member"}); err != nil {
		t.Fatal(err)
	}
	dup = channelInput(other.ID)
	dup.IdempotencyKey = "after-member-stop"
	dup.ExternalPositionID = "new-id"
	if _, err = s.Configure(ctx, configureActor(), dup); !errors.Is(err, ErrNotEnabled) {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000005_channel_positions.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.db.Exec(string(down)); err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile("../../migrations/000005_channel_positions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.db.Exec(string(up)); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPositionReadinessProjection(t *testing.T) {
	repo, id := channelPositionDB(t)
	ctx := context.Background()
	s := PositionService{Repository: repo}
	read := func(want string) {
		t.Helper()
		page, err := s.ListPositions(ctx, "u1", PositionListInput{})
		if err != nil || len(page.Items) != 1 || page.Items[0].ChannelReadiness != want {
			t.Fatal(page, err, want)
		}
	}
	read("WAITING_CONFIGURATION")
	if _, err := (ChannelPositionService{Repository: repo}).Configure(ctx, configureActor(), channelInput(id)); err != nil {
		t.Fatal(err)
	}
	read("WAITING_VERIFICATION")
	// Membership suspension makes an otherwise enabled position unavailable.
	if _, err := (AdminService{Repository: repo}).Execute(ctx, adminActor(), AdminCommand{Action: DisableAction, TargetID: "u1", Reason: "policy", Version: 2, IdempotencyKey: "suspend"}); err != nil {
		t.Fatal(err)
	}
	read("UNAVAILABLE")
}
