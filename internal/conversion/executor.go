package conversion

import (
	"context"
	"errors"
	"time"

	"github.com/kev-chen369/shlms/internal/preview"
)

var ErrChannelRejected = errors.New("channel definitively rejected conversion")

type CreateCommand struct {
	RequestID          string
	TrackingID         string
	ExternalProductID  string
	AccountID          string
	ExternalPositionID string
}

type LookupStatus string

const (
	LookupUnknown   LookupStatus = "UNKNOWN"
	LookupSucceeded LookupStatus = "SUCCEEDED"
	LookupRejected  LookupStatus = "REJECTED"
)

type LookupResult struct {
	Status  LookupStatus
	LinkURL string
}

// LinkGateway must be an approved channel implementation. Lookup must resolve
// the same stable RequestID; this executor never repeats Create after an
// uncertain result, including when Lookup reports UNKNOWN.
type LinkGateway interface {
	Create(context.Context, CreateCommand) (string, error)
	Lookup(context.Context, string) (LookupResult, error)
	ValidateLinkURL(string) bool
}

type Executor struct {
	Repository  Repository
	Eligibility preview.Eligibility
	Previews    PreviewReader
	Requoter    Requoter
	Gateway     LinkGateway
}

func (e Executor) ready() bool {
	return e.Repository.DB != nil && e.Eligibility != nil && e.Previews != nil && e.Requoter != nil && e.Gateway != nil
}

// RunPending is a private worker entry point, intentionally not wired in cmd/api.
func (e Executor) RunPending(ctx context.Context, id string) (Record, error) {
	if !e.ready() {
		return Record{}, ErrUnavailable
	}
	record, err := e.Repository.FindByIDInternal(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if record.Status != "PENDING" {
		return Record{}, ErrStateConflict
	}
	position, err := e.Eligibility.Check(ctx, record.OwnerUserID, record.PositionID)
	if err != nil {
		return Record{}, eligibilityError(err)
	}
	snapshot, err := e.Previews.FindByID(ctx, record.OwnerUserID, record.PreviewID)
	if err != nil {
		return Record{}, err
	}
	if snapshot.PositionID != record.PositionID || snapshot.Scene != record.Scene ||
		snapshot.Channel != "JD" || !snapshot.ExpiresAt.After(time.Now()) {
		return e.Repository.RejectPending(ctx, record.ID, record.Version)
	}
	quote, err := e.Requoter.Requote(ctx, snapshot.ExternalProductID, position)
	if err != nil {
		// No link creation was attempted; a later worker may safely recheck.
		return Record{}, ErrUnavailable
	}
	if !quoteMatches(snapshot, quote) {
		return e.Repository.RejectPending(ctx, record.ID, record.Version)
	}
	if _, err = e.Eligibility.Check(ctx, record.OwnerUserID, record.PositionID); err != nil {
		return Record{}, eligibilityError(err)
	}
	claimed, err := e.Repository.Claim(ctx, record.ID, record.Version, time.Minute)
	if err != nil {
		return Record{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	link, callErr := e.Gateway.Create(callCtx, CreateCommand{
		RequestID: claimed.ChannelRequestID, TrackingID: claimed.TrackingID,
		ExternalProductID: snapshot.ExternalProductID,
		AccountID:         position.AccountID, ExternalPositionID: position.ExternalPositionID,
	})
	// Preserve the uncertain result even when the caller cancels after the
	// external call, so recovery can query rather than send Create again.
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer persistCancel()
	if errors.Is(callErr, ErrChannelRejected) {
		return e.Repository.MarkFinal(persistCtx, claimed.ID, claimed.ChannelRequestID, claimed.Version, "CHANNEL_REJECTED")
	}
	if callErr != nil || !e.Gateway.ValidateLinkURL(link) || !safeHTTPSLink(link) {
		return e.Repository.MarkUncertain(persistCtx, claimed.ID, claimed.ChannelRequestID, claimed.Version)
	}
	return e.Repository.MarkSucceeded(persistCtx, claimed.ID, claimed.ChannelRequestID, claimed.Version, link)
}

// Recover queries the channel by stable request ID. It never invokes Create.
func (e Executor) Recover(ctx context.Context, id string) (Record, error) {
	if e.Repository.DB == nil || e.Gateway == nil {
		return Record{}, ErrUnavailable
	}
	record, err := e.Repository.FindByIDInternal(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if record.Status != "FAILED_RETRYABLE" &&
		(record.Status != "PROCESSING" || record.LeaseExpiresAt == nil || record.LeaseExpiresAt.After(time.Now())) {
		return Record{}, ErrStateConflict
	}
	if record.ChannelRequestID == "" {
		return Record{}, ErrInvalid
	}
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := e.Gateway.Lookup(queryCtx, record.ChannelRequestID)
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer persistCancel()
	if err != nil || result.Status == LookupUnknown {
		if record.Status == "PROCESSING" {
			return e.Repository.MarkUncertain(persistCtx, record.ID, record.ChannelRequestID, record.Version)
		}
		return record, nil
	}
	switch result.Status {
	case LookupSucceeded:
		if !e.Gateway.ValidateLinkURL(result.LinkURL) || !safeHTTPSLink(result.LinkURL) {
			return record, ErrUnavailable
		}
		return e.Repository.MarkSucceeded(persistCtx, record.ID, record.ChannelRequestID, record.Version, result.LinkURL)
	case LookupRejected:
		return e.Repository.MarkFinal(persistCtx, record.ID, record.ChannelRequestID, record.Version, "CHANNEL_REJECTED")
	default:
		return record, ErrUnavailable
	}
}
