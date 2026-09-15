package jd

import (
	"context"
	"fmt"

	"github.com/kev-chen369/shlms/internal/channel"
)

type Config struct {
	PositionID      string
	SubUnionEnabled bool
}

type ClientRequest struct {
	ExternalProductID string
	PositionID        string
	SubUnionID        string
}

type ClientResponse struct {
	URL       string
	SchemeURL string
}

type Client interface {
	GeneratePromotionLink(context.Context, ClientRequest) (ClientResponse, error)
}

type Adapter struct {
	client Client
	config Config
}

func NewAdapter(client Client, config Config) Adapter {
	return Adapter{client: client, config: config}
}

func (a Adapter) CreatePromotionLink(ctx context.Context, request channel.PromotionRequest) (channel.PromotionLink, error) {
	clientRequest := ClientRequest{
		ExternalProductID: request.ExternalProductID,
		PositionID:        a.config.PositionID,
	}
	if a.config.SubUnionEnabled {
		clientRequest.SubUnionID = request.TrackingID
	}

	response, err := a.client.GeneratePromotionLink(ctx, clientRequest)
	if err != nil {
		return channel.PromotionLink{}, fmt.Errorf("generate JD promotion link: %w", err)
	}
	return channel.PromotionLink{
		URL:       response.URL,
		SchemeURL: response.SchemeURL,
	}, nil
}
