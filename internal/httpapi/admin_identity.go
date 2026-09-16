package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/auth"
	"github.com/kev-chen369/shlms/internal/promoter"
)

func resolveAdmin(w http.ResponseWriter, r *http.Request, resolver AdminResolver) (promoter.AdminActor, bool) {
	actor, err := resolver.ResolveAdmin(r)
	if errors.Is(err, auth.ErrUnavailable) {
		writeError(w, 503, "ADMIN_AUTH_UNAVAILABLE", "administrator authorization is unavailable")
		return promoter.AdminActor{}, false
	}
	if err != nil || strings.TrimSpace(actor.ID) == "" {
		writeError(w, 401, "UNAUTHORIZED", "administrator authentication required")
		return promoter.AdminActor{}, false
	}
	return actor, true
}
