package channel

import "context"

type PromotionRequest struct {
	ExternalProductID string
	TrackingID        string
}

type PromotionLink struct {
	URL       string
	SchemeURL string
}

type PromotionLinker interface {
	CreatePromotionLink(context.Context, PromotionRequest) (PromotionLink, error)
}
