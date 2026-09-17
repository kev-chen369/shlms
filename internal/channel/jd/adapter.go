package jd

import (
	"context"
	"errors"
	"strings"

	"github.com/kev-chen369/shlms/internal/channel"
)

type Config struct {
	PositionID      string
	SubUnionEnabled bool
}

type ClientRequest struct {
	ExternalProductID string
	MaterialID        string
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

var ErrNotConfigured = errors.New("JD promotion client and position must be configured")
var ErrTrackingRequired = errors.New("tracking ID is required when JD sub-union attribution is enabled")

// Preserve errors.Is/As for internal handling without exposing a provider's
// raw response, request URL or credentials in ordinary log/error messages.
type providerError struct{ cause error }

func (e providerError) Error() string { return "JD promotion provider unavailable" }
func (e providerError) Unwrap() error { return e.cause }

func NewAdapter(client Client, config Config) Adapter {
	return Adapter{client: client, config: config}
}

func (a Adapter) CreatePromotionLink(ctx context.Context, request channel.PromotionRequest) (channel.PromotionLink, error) {
	if a.client == nil || strings.TrimSpace(a.config.PositionID) == "" {
		return channel.PromotionLink{}, ErrNotConfigured
	}
	material, err := ProductMaterial(request.ExternalProductID)
	if err != nil {
		return channel.PromotionLink{}, err
	}
	if a.config.SubUnionEnabled && strings.TrimSpace(request.TrackingID) == "" {
		return channel.PromotionLink{}, ErrTrackingRequired
	}
	if err := ctx.Err(); err != nil {
		return channel.PromotionLink{}, err
	}
	clientRequest := ClientRequest{
		ExternalProductID: request.ExternalProductID,
		MaterialID:        material,
		PositionID:        a.config.PositionID,
	}
	if a.config.SubUnionEnabled {
		clientRequest.SubUnionID = request.TrackingID
	}

	response, err := a.client.GeneratePromotionLink(ctx, clientRequest)
	if err != nil {
		return channel.PromotionLink{}, providerError{cause: err}
	}
	return channel.PromotionLink{
		URL:       response.URL,
		SchemeURL: response.SchemeURL,
	}, nil
}
