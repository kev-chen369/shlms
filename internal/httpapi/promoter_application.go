package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/kev-chen369/shlms/internal/promoter"
)

type PromoterApplicant interface {
	SubmitApplication(context.Context, promoter.SubmitInput) (promoter.Application, error)
}

func promoterApplicationHandler(d Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, err := d.Users.ResolveUserID(r)
		if err != nil || strings.TrimSpace(userID) == "" {
			writeError(w, 401, "UNAUTHORIZED", "authentication required")
			return
		}
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || contentType != "application/json" {
			writeError(w, 415, "UNSUPPORTED_MEDIA_TYPE", "application/json is required")
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			writeError(w, 400, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
			return
		}
		var body struct {
			DisplayName      string `json:"displayName"`
			Scene            string `json:"scene"`
			AgreementVersion string `json:"agreementVersion"`
			Agreed           bool   `json:"agreed"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&body)
		if err == nil {
			var extra any
			err = decoder.Decode(&extra)
			if errors.Is(err, io.EOF) {
				err = nil
			} else if err == nil {
				err = errors.New("multiple JSON values")
			}
		}
		if err != nil {
			var large *http.MaxBytesError
			if errors.As(err, &large) {
				writeError(w, 413, "REQUEST_TOO_LARGE", "request exceeds size limit")
			} else {
				writeError(w, 400, "INVALID_REQUEST", "request body is invalid")
			}
			return
		}
		a, err := d.PromoterApplications.SubmitApplication(r.Context(), promoter.SubmitInput{UserID: userID, IdempotencyKey: key, DisplayName: body.DisplayName, Scene: body.Scene, AgreementVersion: body.AgreementVersion, Agreed: body.Agreed})
		if err != nil {
			switch {
			case errors.Is(err, promoter.ErrAgreement):
				writeError(w, 422, "AGREEMENT_REQUIRED", "accept the current agreement")
			case errors.Is(err, promoter.ErrInvalidInput):
				writeError(w, 400, "INVALID_REQUEST", "application input is invalid")
			case errors.Is(err, promoter.ErrIdempotencyConflict):
				writeError(w, 409, "IDEMPOTENCY_CONFLICT", "key belongs to a different application")
			case errors.Is(err, promoter.ErrTransition):
				writeError(w, 409, "APPLICATION_NOT_ALLOWED", "current membership does not allow a new application")
			default:
				writeError(w, 503, "PROMOTER_UNAVAILABLE", "application service is unavailable")
			}
			return
		}
		if a.UserID != userID {
			writeError(w, 503, "PROMOTER_UNAVAILABLE", "application service is unavailable")
			return
		}
		// This is the stable submission receipt, not the current review decision.
		writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": map[string]any{"applicationId": a.ID, "status": promoter.Pending, "consentedAt": a.ConsentedAt}})
	}
}
