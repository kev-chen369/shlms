package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kev-chen369/shlms/internal/promotion"
	"github.com/kev-chen369/shlms/internal/tracking"
)

type PromotionCreator interface {
	CreateLink(context.Context, promotion.CreateLinkInput) (promotion.LinkResult, error)
}

type UserResolver interface {
	ResolveUserID(*http.Request) (string, error)
}

type Dependencies struct {
	ChannelPositions           ChannelPositionConfigurator
	Positions                  PositionManager
	PromoterAdminList          PromoterAdminLister
	Admins                     AdminResolver
	PromoterAdmin              PromoterAdministrator
	Promotion                  PromotionCreator
	Users                      UserResolver
	Promoter                   PromoterReader
	PromoterApplications       PromoterApplicant
	PromoterCurrentApplication PromoterApplicationReader
}

type promotionLinkRequest struct {
	Channel          string `json:"channel"`
	ChannelProductID string `json:"channelProductId"`
	Source           string `json:"source"`
}

func promotionLinkHandler(dependencies Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		idempotencyKey := request.Header.Get("Idempotency-Key")
		if idempotencyKey == "" {
			writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
			return
		}

		userID, err := dependencies.Users.ResolveUserID(request)
		if err != nil || userID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		var body promotionLinkRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body is invalid")
			return
		}

		result, err := dependencies.Promotion.CreateLink(request.Context(), promotion.CreateLinkInput{
			IdempotencyKey:    idempotencyKey,
			UserID:            userID,
			Channel:           tracking.Channel(body.Channel),
			ExternalProductID: body.ChannelProductID,
			Source:            body.Source,
		})
		if err != nil {
			switch {
			case errors.Is(err, tracking.ErrConflict):
				writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "idempotency key belongs to a different request")
			case errors.Is(err, tracking.ErrInvalidInput):
				writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "promotion request is invalid")
			case errors.Is(err, promotion.ErrUnsupportedChannel):
				writeError(w, http.StatusUnprocessableEntity, "UNSUPPORTED_CHANNEL", "promotion channel is unsupported")
			default:
				writeError(w, http.StatusBadGateway, "CHANNEL_UNAVAILABLE", "promotion channel is unavailable")
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "success",
			"data": map[string]string{
				"trackingId":   result.TrackingID,
				"promotionUrl": result.URL,
				"schemeUrl":    result.SchemeURL,
			},
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"code":    code,
		"message": message,
		"data":    nil,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
