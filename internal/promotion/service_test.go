package promotion

import (
	"context"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/channel"
	"github.com/kev-chen369/shlms/internal/tracking"
)

type orderedTracker struct {
	called *bool
	record tracking.Record
}

func (t orderedTracker) Create(_ context.Context, _ tracking.CreateInput) (tracking.Record, error) {
	*t.called = true
	return t.record, nil
}

type orderCheckingLinker struct {
	trackingCreated *bool
	request         channel.PromotionRequest
}

func (l *orderCheckingLinker) CreatePromotionLink(_ context.Context, request channel.PromotionRequest) (channel.PromotionLink, error) {
	if !*l.trackingCreated {
		panic("promotion provider called before tracking creation")
	}
	l.request = request
	return channel.PromotionLink{URL: "https://provider.example/promotion"}, nil
}

func TestCreateLinkPersistsTrackingBeforeCallingProvider(t *testing.T) {
	trackingCreated := false
	record := tracking.Record{
		ID:                "TRK-1",
		IdempotencyKey:    "request-1",
		UserID:            "user-1",
		Channel:           tracking.ChannelJD,
		ExternalProductID: "sku-1",
		Source:            "product_detail",
		CreatedAt:         time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
	}
	linker := &orderCheckingLinker{trackingCreated: &trackingCreated}
	service := NewService(
		orderedTracker{called: &trackingCreated, record: record},
		map[tracking.Channel]channel.PromotionLinker{tracking.ChannelJD: linker},
	)

	got, err := service.CreateLink(context.Background(), CreateLinkInput{
		IdempotencyKey:    "request-1",
		UserID:            "user-1",
		Channel:           tracking.ChannelJD,
		ExternalProductID: "sku-1",
		Source:            "product_detail",
	})

	if err != nil {
		t.Fatalf("CreateLink returned error: %v", err)
	}
	if linker.request.TrackingID != "TRK-1" || linker.request.ExternalProductID != "sku-1" {
		t.Fatalf("provider request = %#v, want tracking and product IDs", linker.request)
	}
	if got.TrackingID != "TRK-1" || got.URL != "https://provider.example/promotion" {
		t.Fatalf("result = %#v, want tracking ID and promotion URL", got)
	}
}
