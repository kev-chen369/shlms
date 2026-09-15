package promotion

import (
	"context"
	"errors"

	"github.com/kev-chen369/shlms/internal/channel"
	"github.com/kev-chen369/shlms/internal/tracking"
)

var ErrUnsupportedChannel = errors.New("unsupported promotion channel")

type Tracker interface {
	Create(context.Context, tracking.CreateInput) (tracking.Record, error)
}

type CreateLinkInput struct {
	IdempotencyKey    string
	UserID            string
	Channel           tracking.Channel
	ExternalProductID string
	Source            string
}

type LinkResult struct {
	TrackingID string
	URL        string
	SchemeURL  string
}

type Service struct {
	tracker Tracker
	linkers map[tracking.Channel]channel.PromotionLinker
}

func NewService(tracker Tracker, linkers map[tracking.Channel]channel.PromotionLinker) Service {
	return Service{tracker: tracker, linkers: linkers}
}

func (s Service) CreateLink(ctx context.Context, input CreateLinkInput) (LinkResult, error) {
	record, err := s.tracker.Create(ctx, tracking.CreateInput{
		IdempotencyKey:    input.IdempotencyKey,
		UserID:            input.UserID,
		Channel:           input.Channel,
		ExternalProductID: input.ExternalProductID,
		Source:            input.Source,
	})
	if err != nil {
		return LinkResult{}, err
	}

	linker, ok := s.linkers[input.Channel]
	if !ok {
		return LinkResult{}, ErrUnsupportedChannel
	}
	link, err := linker.CreatePromotionLink(ctx, channel.PromotionRequest{
		ExternalProductID: input.ExternalProductID,
		TrackingID:        record.ID,
	})
	if err != nil {
		return LinkResult{}, err
	}
	return LinkResult{
		TrackingID: record.ID,
		URL:        link.URL,
		SchemeURL:  link.SchemeURL,
	}, nil
}
