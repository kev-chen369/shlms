package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type PromoterReader interface {
	GetProfile(context.Context, string) (promoter.Profile, error)
}

func promoterProfileHandler(dependencies Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := dependencies.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		profile, err := dependencies.Promoter.GetProfile(r.Context(), userID)
		if err != nil || profile.UserID != userID {
			writeError(w, http.StatusServiceUnavailable, "PROMOTER_UNAVAILABLE", "promoter profile is unavailable")
			return
		}
		// Approval notes are internal. Only a rejection/disable reason intended
		// for the applicant is exposed; admin audit notes must be stored separately.
		reason := ""
		if profile.Status == promoter.Rejected || profile.Status == promoter.Disabled {
			reason = profile.Reason
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code": 0, "message": "success",
			"data": map[string]any{"status": profile.Status, "reason": reason, "applicationId": profile.ApplicationID, "capabilities": profile.Permissions()},
		})
	}
}
