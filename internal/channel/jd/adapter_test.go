package jd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/channel"
)

type recordingClient struct {
	request  ClientRequest
	response ClientResponse
	err      error
	calls    int
}

func (c *recordingClient) GeneratePromotionLink(_ context.Context, request ClientRequest) (ClientResponse, error) {
	c.request = request
	c.calls++
	return c.response, c.err
}

func TestCreatePromotionLinkMapsTrackingToSubUnion(t *testing.T) {
	client := &recordingClient{response: ClientResponse{
		URL:       "https://provider.example/promotion",
		SchemeURL: "openapp://promotion",
	}}
	adapter := NewAdapter(client, Config{PositionID: "position-1", SubUnionEnabled: true})

	got, err := adapter.CreatePromotionLink(context.Background(), channel.PromotionRequest{
		ExternalProductID: "123456",
		TrackingID:        "TRK-1",
	})

	if err != nil {
		t.Fatalf("CreatePromotionLink returned error: %v", err)
	}
	wantRequest := ClientRequest{
		ExternalProductID: "123456",
		MaterialID:        "https://item.jd.com/123456.html",
		PositionID:        "position-1",
		SubUnionID:        "TRK-1",
	}
	if client.request != wantRequest {
		t.Fatalf("client request = %#v, want %#v", client.request, wantRequest)
	}
	if got.URL != client.response.URL || got.SchemeURL != client.response.SchemeURL {
		t.Fatalf("link = %#v, want provider URLs", got)
	}
}

func TestCreatePromotionLinkOmitsSubUnionWhenDisabled(t *testing.T) {
	client := &recordingClient{response: ClientResponse{URL: "https://provider.example/promotion"}}
	adapter := NewAdapter(client, Config{PositionID: "position-1"})

	_, err := adapter.CreatePromotionLink(context.Background(), channel.PromotionRequest{
		ExternalProductID: "123456",
		TrackingID:        "TRK-1",
	})

	if err != nil {
		t.Fatalf("CreatePromotionLink returned error: %v", err)
	}
	if client.request.SubUnionID != "" {
		t.Fatalf("SubUnionID = %q, want blank", client.request.SubUnionID)
	}
}

func TestCreatePromotionLinkWrapsProviderError(t *testing.T) {
	providerErr := errors.New("provider unavailable: secret=test-only-sensitive-value")
	client := &recordingClient{err: providerErr}
	adapter := NewAdapter(client, Config{PositionID: "position-1"})

	_, err := adapter.CreatePromotionLink(context.Background(), channel.PromotionRequest{
		ExternalProductID: "123456",
		TrackingID:        "TRK-1",
	})

	if !errors.Is(err, providerErr) {
		t.Fatalf("error = %v, want wrapped provider error", err)
	}
	if strings.Contains(err.Error(), "test-only-sensitive-value") {
		t.Fatal("provider diagnostic leaked through adapter error")
	}
}

func TestAdapterRejectsBeforeCallingProvider(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  Config
		request channel.PromotionRequest
		want    error
	}{
		{"missing position", Config{}, channel.PromotionRequest{ExternalProductID: "123"}, ErrNotConfigured},
		{"invalid material", Config{PositionID: "position-1"}, channel.PromotionRequest{ExternalProductID: "https://evil.example/123"}, ErrInvalidMaterial},
		{"missing tracking", Config{PositionID: "position-1", SubUnionEnabled: true}, channel.PromotionRequest{ExternalProductID: "123"}, ErrTrackingRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &recordingClient{}
			_, err := NewAdapter(client, tc.config).CreatePromotionLink(context.Background(), tc.request)
			if !errors.Is(err, tc.want) || client.calls != 0 {
				t.Fatalf("error=%v calls=%d", err, client.calls)
			}
		})
	}
	_, err := NewAdapter(nil, Config{PositionID: "position-1"}).CreatePromotionLink(context.Background(), channel.PromotionRequest{ExternalProductID: "123"})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil client: %v", err)
	}
}

func TestAdapterDoesNotCallProviderAfterCancellation(t *testing.T) {
	client := &recordingClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewAdapter(client, Config{PositionID: "position-1"}).CreatePromotionLink(ctx, channel.PromotionRequest{ExternalProductID: "123"})
	if !errors.Is(err, context.Canceled) || client.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, client.calls)
	}
}

func TestAdapterAcceptsProductURLAndKeepsTrackingPrivateByDefault(t *testing.T) {
	client := &recordingClient{response: ClientResponse{URL: "https://provider.example/promotion"}}
	_, err := NewAdapter(client, Config{PositionID: "position-1"}).CreatePromotionLink(context.Background(), channel.PromotionRequest{
		ExternalProductID: "https://item.jd.com/12345678901234567890.html", TrackingID: "private-tracking-id",
	})
	if err != nil || client.calls != 1 || client.request.MaterialID != "https://item.jd.com/12345678901234567890.html" || client.request.SubUnionID != "" {
		t.Fatalf("unexpected request: %+v err=%v calls=%d", client.request, err, client.calls)
	}
}
