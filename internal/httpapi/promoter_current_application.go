package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type PromoterApplicationReader interface {
	GetCurrentApplication(context.Context, string) (promoter.CurrentApplication, error)
}

func promoterCurrentApplicationHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		current, err := d.PromoterCurrentApplication.GetCurrentApplication(r.Context(), userID)
		if errors.Is(err, promoter.ErrNotFound) {
			writeError(w, 404, "APPLICATION_NOT_FOUND", "no application found")
			return
		}
		if err != nil || current.Application.UserID != userID {
			writeError(w, 503, "PROMOTER_UNAVAILABLE", "application service is unavailable")
			return
		}
		reason := ""
		if current.Status == promoter.Rejected || current.Status == promoter.Disabled {
			reason = current.Reason
		}
		a := current.Application
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{
			"applicationId": a.ID, "displayName": a.DisplayName, "scene": a.Scene, "agreementVersion": a.AgreementVersion, "consentedAt": a.ConsentedAt,
			"status": current.Status, "reason": reason, "canReapply": current.Status == promoter.Rejected,
		}})
	}
}
