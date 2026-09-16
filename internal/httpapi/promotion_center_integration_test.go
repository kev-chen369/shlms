package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/promoter"
)

func promotionCenterDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN is not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	schema := fmt.Sprintf("promotion_flow_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	for _, name := range []string{"000002_promoter_applications.up.sql", "000003_promoter_admin_audits.up.sql", "000004_promotion_positions.up.sql", "000005_channel_positions.up.sql"} {
		b, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatal(name, err)
		}
	}
	return db
}

func TestPromotionCenterMembershipToPositionFlow(t *testing.T) {
	db := promotionCenterDB(t)
	repo := promoter.NewPostgresRepository(db)
	userID := "u1"
	users := identityFunc(func(*http.Request) (string, error) { return userID, nil })
	admins := adminResolverFunc(func(*http.Request) (promoter.AdminActor, error) {
		return promoter.AdminActor{ID: "admin", Permissions: map[string]bool{
			promoter.ReviewPermission: true, promoter.DisablePermission: true, promoter.ReadPermission: true, promoter.ConfigureChannelPermission: true,
		}}, nil
	})
	d := Dependencies{
		Users: users, Admins: admins, Promoter: promoter.Service{Repository: repo},
		PromoterApplications:       promoter.ApplicationService{Repository: repo, AgreementVersion: "v1"},
		PromoterCurrentApplication: promoter.CurrentApplicationService{Repository: repo},
		PromoterAdmin:              promoter.AdminService{Repository: repo}, PromoterAdminList: promoter.AdminListService{Repository: repo},
		Positions: promoter.PositionService{Repository: repo}, ChannelPositions: promoter.ChannelPositionService{Repository: repo},
	}
	router := NewRouterWithDependencies(d)
	call := func(method, path, body, key string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s = %d: %s", method, path, w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	data := func(response map[string]any) map[string]any {
		t.Helper()
		m, ok := response["data"].(map[string]any)
		if !ok {
			t.Fatal(response)
		}
		return m
	}
	profile := data(call("GET", "/api/v1/promoter/profile", "", "", 200))
	if profile["status"] != "NOT_APPLIED" {
		t.Fatal(profile)
	}
	apply := `{"displayName":"Alice","scene":"group","agreementVersion":"v1","agreed":true}`
	first := data(call("POST", "/api/v1/promoter/applications", apply, "apply-1", 200))
	replay := data(call("POST", "/api/v1/promoter/applications", apply, "apply-1", 200))
	if first["applicationId"] != replay["applicationId"] {
		t.Fatal(first, replay)
	}
	appID := first["applicationId"].(string)
	current := data(call("GET", "/api/v1/promoter/applications/current", "", "", 200))
	if current["status"] != "PENDING" || current["applicationId"] != appID {
		t.Fatal(current)
	}
	queue := data(call("GET", "/admin/v1/promoter-applications?status=PENDING", "", "", 200))
	if len(queue["items"].([]any)) != 1 {
		t.Fatal(queue)
	}
	approved := data(call("POST", "/admin/v1/promoter-applications/"+appID+"/review", `{"approve":true,"reason":"approved","version":1}`, "review-1", 200))
	if approved["status"] != "ENABLED" || approved["version"] != float64(2) {
		t.Fatal(approved)
	}
	call("POST", "/admin/v1/promoter-applications/"+appID+"/review", `{"approve":true,"reason":"approved","version":1}`, "review-2", 409)
	create := `{"name":"group goods","scene":"group","isDefault":true}`
	p1 := data(call("POST", "/api/v1/promotion-positions", create, "create-1", 200))
	p1ID := p1["id"].(string)
	if p1["channels"].([]any)[0].(map[string]any)["readiness"] != "WAITING_CONFIGURATION" {
		t.Fatal(p1)
	}
	p2 := data(call("POST", "/api/v1/promotion-positions", `{"name":"moments","scene":"social"}`, "create-2", 200))
	p2ID := p2["id"].(string)
	call("POST", "/api/v1/promotion-positions/"+p2ID+"/default", `{"version":1}`, "default-2", 200)
	call("PATCH", "/api/v1/promotion-positions/"+p1ID, `{"name":"stale","scene":"group","version":1}`, "edit-stale", 409)
	config := data(call("PUT", "/admin/v1/promotion-positions/"+p2ID+"/channels/JD", `{"accountId":"account-1","externalPositionId":"external-1","version":0}`, "channel-1", 200))
	if config["status"] != "PENDING_VERIFICATION" {
		t.Fatal(config)
	}
	positions := data(call("GET", "/api/v1/promotion-positions", "", "", 200))
	items := positions["items"].([]any)
	if len(items) != 2 {
		t.Fatal(positions)
	}
	for _, raw := range items {
		p := raw.(map[string]any)
		if p["id"] == p2ID {
			if p["canConvert"] != false || p["channels"].([]any)[0].(map[string]any)["readiness"] != "WAITING_VERIFICATION" {
				t.Fatal(p)
			}
		}
	}
	userID = "u2"
	other := data(call("GET", "/api/v1/promotion-positions", "", "", 200))
	if len(other["items"].([]any)) != 0 {
		t.Fatal("other user's positions leaked", other)
	}
	call("GET", "/api/v1/promoter/applications/current", "", "", 404)
	call("POST", "/api/v1/promotion-positions/"+p2ID+"/disable", `{"version":2}`, "cross-user", 403)
	userID = "u1"
	call("POST", "/admin/v1/promoters/u1/disable", `{"reason":"policy","version":2}`, "disable-member", 200)
	call("POST", "/api/v1/promotion-positions", `{"name":"blocked","scene":"group"}`, "blocked-create", 403)
	positions = data(call("GET", "/api/v1/promotion-positions", "", "", 200))
	items = positions["items"].([]any)
	if len(items) != 2 {
		t.Fatal("history lost", positions)
	}
	for _, raw := range items {
		p := raw.(map[string]any)
		if p["channels"].([]any)[0].(map[string]any)["readiness"] != "UNAVAILABLE" {
			t.Fatal(p)
		}
	}
	var applications, positionCount, audits int
	if err := db.QueryRowContext(context.Background(), `SELECT (SELECT count(*) FROM promoter_applications),(SELECT count(*) FROM promotion_positions),(SELECT count(*) FROM promoter_admin_audits)`).Scan(&applications, &positionCount, &audits); err != nil || applications != 1 || positionCount != 2 || audits != 2 {
		t.Fatal(applications, positionCount, audits, err)
	}
}
