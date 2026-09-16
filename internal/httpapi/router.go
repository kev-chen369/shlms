package httpapi

import (
	"encoding/json"
	"net/http"
)

func NewRouter() http.Handler {
	return NewRouterWithDependencies(Dependencies{})
}

func NewRouterWithDependencies(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	if dependencies.Promotion != nil && dependencies.Users != nil {
		mux.HandleFunc("POST /api/v1/promotions/link", promotionLinkHandler(dependencies))
	}
	if dependencies.Promoter != nil && dependencies.Users != nil {
		mux.HandleFunc("GET /api/v1/promoter/profile", promoterProfileHandler(dependencies))
	}
	return mux
}
