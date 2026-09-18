package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/kev-chen369/shlms/web"
)

func NewRouter() http.Handler {
	return NewRouterWithDependencies(Dependencies{})
}

func NewRouterWithDependencies(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /app/", http.StripPrefix("/app/", web.Handler()))
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	if dependencies.Coupons != nil {
		mux.HandleFunc("GET /api/v1/coupons", couponListHandler(dependencies))
		mux.HandleFunc("GET /api/v1/coupons/{id}", couponDetailHandler(dependencies))
		mux.HandleFunc("GET /api/v1/coupons/{id}/products", couponProductsHandler(dependencies))
	}
	if dependencies.CouponCities != nil {
		mux.HandleFunc("GET /api/v1/coupon-cities", couponCitiesHandler(dependencies))
	}
	if dependencies.Claims != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/coupons/{id}/claims", couponClaimHandler(dependencies))
	}
	if dependencies.Outbound != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/coupons/{id}/outbound", couponOutboundHandler(dependencies))
	}
	if dependencies.ClaimReader != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/coupon-claims/{id}", couponClaimStatusHandler(dependencies))
	}
	if dependencies.Promotion != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/promotions/link", promotionLinkHandler(dependencies))
	}
	if dependencies.Preview != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/promotions/preview", promotionPreviewHandler(dependencies))
	}
	if dependencies.Conversion != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/promotions/convert", promotionConvertHandler(dependencies))
	}
	if dependencies.ConversionReader != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promotions/convert/{id}", promotionConvertStatusHandler(dependencies))
	}
	if dependencies.ShareArtifacts != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promotions/convert/{id}/share-artifacts", promotionShareArtifactHandler(dependencies))
	}
	if dependencies.Promoter != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promoter/profile", promoterProfileHandler(dependencies))
	}
	if dependencies.Orders != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promoter/orders", promoterOrderListHandler(dependencies))
		mux.HandleFunc("GET /api/v1/promoter/orders/{id}", promoterOrderDetailHandler(dependencies))
	}
	if dependencies.PromoterApplications != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/promoter/applications", promoterApplicationHandler(dependencies))
	}
	if dependencies.PromoterCurrentApplication != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promoter/applications/current", promoterCurrentApplicationHandler(dependencies))
	}
	if dependencies.Admins != nil && dependencies.PromoterAdmin != nil {
		mux.HandleFunc("POST /admin/v1/promoter-applications/{id}/review", promoterAdminHandler(dependencies, "REVIEW"))
		mux.HandleFunc("POST /admin/v1/promoters/{id}/disable", promoterAdminHandler(dependencies, "DISABLE"))
	}
	if dependencies.Admins != nil && dependencies.PromoterAdminList != nil {
		mux.HandleFunc("GET /admin/v1/promoter-applications", promoterAdminListHandler(dependencies))
	}
	if dependencies.Admins != nil && dependencies.ChannelPositions != nil {
		mux.HandleFunc("PUT /admin/v1/promotion-positions/{id}/channels/{channel}", channelPositionConfigHandler(dependencies))
	}
	if dependencies.Positions != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promotion-positions", positionListHandler(dependencies))
		mux.HandleFunc("POST /api/v1/promotion-positions", positionWriteHandler(dependencies, "CREATE"))
		mux.HandleFunc("PATCH /api/v1/promotion-positions/{id}", positionWriteHandler(dependencies, "EDIT"))
		mux.HandleFunc("POST /api/v1/promotion-positions/{id}/default", positionWriteHandler(dependencies, "DEFAULT"))
		mux.HandleFunc("POST /api/v1/promotion-positions/{id}/disable", positionWriteHandler(dependencies, "DISABLE"))
	}
	return mux
}
