package jd

import (
	"context"
	"errors"
	"testing"

	"github.com/kev-chen369/shlms/internal/channel"
)

type recordingClient struct {
	request  ClientRequest
	response ClientResponse
	err      error
}

func (c *recordingClient) GeneratePromotionLink(_ context.Context, request ClientRequest) (ClientResponse, error) {
	c.request = request
	return c.response, c.err
}

func TestCreatePromotionLinkMapsTrackingToSubUnion(t *testing.T) {
	client := &recordingClient{response: ClientResponse{
		URL:       "https://provider.example/promotion",
		SchemeURL: "openapp://promotion",
	}}
	adapter := NewAdapter(client, Config{PositionID: "position-1", SubUnionEnabled: true})

	got, err := adapter.CreatePromotionLink(context.Background(), channel.PromotionRequest{
		ExternalProductID: "sku-1",
		TrackingID:        "TRK-1",
	})

	if err != nil {
		t.Fatalf("CreatePromotionLink returned error: %v", err)
	}
	wantRequest := ClientRequest{
		ExternalProductID: "sku-1",
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
		ExternalProductID: "sku-1",
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
	providerErr := errors.New("provider unavailable")
	client := &recordingClient{err: providerErr}
	adapter := NewAdapter(client, Config{PositionID: "position-1"})

	_, err := adapter.CreatePromotionLink(context.Background(), channel.PromotionRequest{
		ExternalProductID: "sku-1",
		TrackingID:        "TRK-1",
	})

	if !errors.Is(err, providerErr) {
		t.Fatalf("error = %v, want wrapped provider error", err)
	}
}
