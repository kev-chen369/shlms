package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/conversion"
)

type shareEventStub struct {
	input conversion.ShareEventInput
	err   error
}

func (s *shareEventStub) Record(_ context.Context, in conversion.ShareEventInput) (conversion.ShareEvent, error) {
	s.input = in
	return conversion.ShareEvent{EventID: in.EventID, RequestID: in.RequestID, Action: in.Action, ArtifactType: in.ArtifactType, Scene: in.Scene, RecordedAt: time.Now()}, s.err
}

func TestShareEventRoute(t *testing.T) {
	stub := &shareEventStub{}
	d := Dependencies{Users: fixedUserResolver{userID: "u1"}, ShareEvents: stub}
	call := func(body string) (int, string) {
		r := httptest.NewRequest("POST", "/api/v1/promotions/convert/c1/share-events", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		NewRouterWithDependencies(d).ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	good := `{"eventId":"event-1","artifactType":"link","scene":"home","action":"COPY_REPORTED"}`
	code, body := call(good)
	if code != 200 || stub.input.OwnerUserID != "u1" || stub.input.RequestID != "c1" || strings.Contains(body, "delivered") || strings.Contains(body, "success") {
		t.Fatal(code, body, stub.input)
	}
	for _, bad := range []string{`{"eventId":"e","artifactType":"link","scene":"home","action":"COPY_REPORTED","ownerUserId":"u2"}`, `{"eventId":"e","artifactType":"link","scene":"home","action":"COPY_REPORTED","targetUrl":"https://evil.example"}`, good + good} {
		code, _ = call(bad)
		if code != 400 {
			t.Fatal(code, bad)
		}
	}
	stub.err = conversion.ErrIdempotencyConflict
	code, _ = call(good)
	if code != 409 {
		t.Fatal(code)
	}
	stub.err = errors.New("private database detail")
	code, body = call(good)
	if code != 503 || strings.Contains(body, "private database detail") {
		t.Fatal(code, body)
	}
	d.Users = fixedUserResolver{userID: ""}
	code, _ = call(good)
	if code != 401 {
		t.Fatal(code)
	}
}
