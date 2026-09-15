package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/promotion"
	"github.com/kev-chen369/shlms/internal/tracking"
)

type fixedUserResolver struct {
	userID string
}

func (r fixedUserResolver) ResolveUserID(*http.Request) (string, error) {
	return r.userID, nil
}

type promotionCreatorStub struct {
	result promotion.LinkResult
	err    error
	input  promotion.CreateLinkInput
}

func (s *promotionCreatorStub) CreateLink(_ context.Context, input promotion.CreateLinkInput) (promotion.LinkResult, error) {
	s.input = input
	return s.result, s.err
}

func TestPromotionLinkReturnsTrackingAndURL(t *testing.T) {
	creator := &promotionCreatorStub{result: promotion.LinkResult{
		TrackingID: "TRK-1",
		URL:        "https://provider.example/promotion",
	}}
	router := NewRouterWithDependencies(Dependencies{
		Promotion: creator,
		Users:     fixedUserResolver{userID: "user-1"},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/link", strings.NewReader(
		"{\"channel\":\"JD\",\"channelProductId\":\"sku-1\",\"source\":\"product_detail\"}",
	))
	request.Header.Set("Idempotency-Key", "request-1")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if creator.input.UserID != "user-1" || creator.input.IdempotencyKey != "request-1" {
		t.Fatalf("service input = %#v, want resolved user and idempotency key", creator.input)
	}
	if body := response.Body.String(); !strings.Contains(body, "TRK-1") || !strings.Contains(body, creator.result.URL) {
		t.Fatalf("body = %s, want tracking ID and promotion URL", body)
	}
}

func TestPromotionLinkRejectsMalformedRequests(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		key      string
		wantCode string
	}{
		{name: "invalid json", body: "{", key: "request-1", wantCode: "INVALID_REQUEST"},
		{name: "missing idempotency", body: "{\"channel\":\"JD\",\"channelProductId\":\"sku-1\"}", wantCode: "IDEMPOTENCY_KEY_REQUIRED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouterWithDependencies(Dependencies{
				Promotion: &promotionCreatorStub{},
				Users:     fixedUserResolver{userID: "user-1"},
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/link", strings.NewReader(test.body))
			request.Header.Set("Idempotency-Key", test.key)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("status/body = %d %s, want 400 and %s", response.Code, response.Body.String(), test.wantCode)
			}
		})
	}
}

func TestPromotionLinkMapsDomainAndProviderErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "unsupported channel", err: promotion.ErrUnsupportedChannel, wantStatus: http.StatusUnprocessableEntity, wantCode: "UNSUPPORTED_CHANNEL"},
		{name: "provider unavailable", err: errors.New("provider unavailable"), wantStatus: http.StatusBadGateway, wantCode: "CHANNEL_UNAVAILABLE"},
		{name: "invalid tracking input", err: tracking.ErrInvalidInput, wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouterWithDependencies(Dependencies{
				Promotion: &promotionCreatorStub{err: test.err},
				Users:     fixedUserResolver{userID: "user-1"},
			})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/link", strings.NewReader(
				"{\"channel\":\"JD\",\"channelProductId\":\"sku-1\"}",
			))
			request.Header.Set("Idempotency-Key", "request-1")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("status/body = %d %s, want %d and %s", response.Code, response.Body.String(), test.wantStatus, test.wantCode)
			}
		})
	}
}
