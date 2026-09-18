package app

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/dbmigrate"
	"github.com/kev-chen369/shlms/internal/material"
)

const runtimeMaterialOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const runtimeMaterialID = "00000000-0000-4000-8000-000000000001"
const runtimeMaterialQuery = "platform=JD&type=PRODUCT&terminal=H5&positionId=runtime-position&scene=home&cityCode=310100&business=food"

func runtimeMaterialFixture(t *testing.T) (*sql.DB, Config, *rsa.PrivateKey, time.Time) {
	t.Helper()
	db := runtimeDB(t)
	public, private := runtimeKeys(t)
	config := Config{PublicKeyPEM: public, Issuer: "https://issuer.example", Audience: "shlms-api", AgreementVersion: "v1", MigrationsDir: "../../migrations", CatalogBindings: []material.CatalogBinding{{Platform: "JD", Type: "PRODUCT", Terminal: "H5", Scene: "home", MediaID: "runtime-media"}}}
	if _, err := dbmigrate.Run(context.Background(), db, config.MigrationsDir); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	for _, sql := range []string{
		`INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED')`,
		`INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status) VALUES('runtime-app',$1,'runtime-key','test','home','v1',now(),'ENABLED')`,
		`UPDATE promoter_profiles SET status='ENABLED',application_id='runtime-app' WHERE user_id=$1`,
		`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES('runtime-position',$1,'main','home','ENABLED',1)`,
	} {
		if _, err := db.Exec(sql, runtimeMaterialOwner); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO channel_capabilities(id,platform,material_type,kind,media_id,position_id,scene,terminal,city_code,business) VALUES('c1111111-1111-4111-8111-111111111111','JD','PRODUCT','CATALOG','runtime-media','runtime-position','home','H5','310100','food')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('c2222222-2222-4222-8222-222222222222','c1111111-1111-4111-8111-111111111111','operator','runtime-media-proof','runtime-source-proof','v1','runtime-call-proof',$1,$2,'runtime-test')`, at.Add(-time.Minute), at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE channel_capabilities SET status='READY',evidence_id='c2222222-2222-4222-8222-222222222222'`); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 2; n++ {
		if _, err := db.Exec(`INSERT INTO promotion_materials(id,platform,material_type,external_material_id,canonical_url,title,status,ends_at,source_updated_at,rule_version,evidence_ref) VALUES($1,'JD','PRODUCT',$2,'https://item.jd.com/private-source.html',$3,'ACTIVE',$4,$5,'v1','runtime-private-material-proof')`, fmt.Sprintf("00000000-0000-4000-8000-%012d", n), fmt.Sprintf("runtime-source-%d", n), fmt.Sprintf("runtime title %d", n), at.Add(time.Hour), at.Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	return db, config, private, at
}

type runtimeMaterialResponse struct {
	Code any             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func getRuntimeMaterial(t *testing.T, server *httptest.Server, path, token string, status int) runtimeMaterialResponse {
	t.Helper()
	return getRuntimeMaterialHTTP(t, server.Client(), server.URL, path, token, status)
}

func getRuntimeMaterialHTTP(t *testing.T, client *http.Client, baseURL, path, token string, status int) runtimeMaterialResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(ctx, "GET", baseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("%s: %d %s", path, resp.StatusCode, raw)
	}
	for _, private := range []string{"private-source", "runtime-source-", "runtime-private-material-proof", "runtime-media-proof", "runtime-call-proof", "canGenerate", "canonicalUrl", "mediaId", "price"} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("leaked %s: %s", private, raw)
		}
	}
	var got runtimeMaterialResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if status == 200 {
		if got.Code != float64(0) {
			t.Fatal(string(raw))
		}
	} else if string(got.Data) != "null" {
		t.Fatal("error returned data", string(raw))
	}
	return got
}

// Catches entrypoint omission of parsed bindings, fallback and startup reloads.
// Real API child processes use only temporary files and a synthetic PG schema.
func TestRuntimeMaterialsDeploymentFileEntrypoint(t *testing.T) {
	db, config, private, _ := runtimeMaterialFixture(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "api")
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer buildCancel()
	if out, err := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "../../cmd/api").CombinedOutput(); err != nil {
		t.Fatalf("build API: %s %v", out, err)
	}
	keyPath := filepath.Join(dir, "public.pem")
	bindingPath := filepath.Join(dir, "bindings.json")
	if err := os.WriteFile(keyPath, config.PublicKeyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	dsn, err := url.Parse(os.Getenv("PG_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	params := dsn.Query()
	params.Set("search_path", schema)
	dsn.RawQuery = params.Encode()
	for _, configured := range []bool{true, false} {
		t.Run(fmt.Sprint(configured), func(t *testing.T) {
			if _, err := db.Exec(`UPDATE channel_capabilities SET status='READY'`); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(bindingPath, []byte(`[{"platform":"JD","type":"PRODUCT","terminal":"H5","scene":"home","mediaId":"runtime-media"}]`), 0600); err != nil {
				t.Fatal(err)
			}
			file := bindingPath
			if !configured {
				file = ""
			}
			// Reserve a loopback-only port; no production port or business DSN is used.
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			if err = listener.Close(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary)
			// Do not inherit deployment secrets or business configuration.
			cmd.Env = []string{"DATABASE_URL=" + dsn.String(), "AUTH_PUBLIC_KEY_FILE=" + keyPath, "AUTH_ISSUER=" + config.Issuer, "AUTH_AUDIENCE=" + config.Audience, "PROMOTER_AGREEMENT_VERSION=" + config.AgreementVersion, "CATALOG_BINDINGS_FILE=" + file, "API_ADDR=" + addr}
			cmd.Dir = "../.."
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); _ = cmd.Wait() }()
			client := &http.Client{Timeout: time.Second}
			baseURL := "http://" + addr
			deadline := time.Now().Add(5 * time.Second)
			for {
				resp, err := client.Get(baseURL + "/healthz")
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == 200 {
						break
					}
				}
				if time.Now().After(deadline) {
					t.Fatal("temporary API did not become ready")
				}
				time.Sleep(20 * time.Millisecond)
			}
			token := runtimeToken(t, private, runtimeMaterialOwner)
			base := "/api/v1/promoter/materials"
			detailPath := base + "/" + runtimeMaterialID + "?" + runtimeMaterialQuery
			read := func(path string) material.Detail {
				t.Helper()
				var detail material.Detail
				got := getRuntimeMaterialHTTP(t, client, baseURL, path, token, 200)
				if err := json.Unmarshal(got.Data, &detail); err != nil {
					t.Fatal(err)
				}
				return detail
			}
			var page material.Page
			got := getRuntimeMaterialHTTP(t, client, baseURL, base+"?"+runtimeMaterialQuery+"&limit=1", token, 200)
			if err := json.Unmarshal(got.Data, &page); err != nil {
				t.Fatal(err)
			}
			detail := read(detailPath)
			if !configured {
				if page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" || page.Capability.Reason != "UNCONFIGURED" || detail.Item != nil || detail.Capability.Reason != "UNCONFIGURED" {
					t.Fatal(page, detail)
				}
				return
			}
			if len(page.Items) != 1 || page.Items[0].ID != runtimeMaterialID || page.NextCursor == "" || !page.Capability.Allowed || detail.Item == nil || detail.Item.ID != runtimeMaterialID {
				t.Fatal(page, detail)
			}
			if err := os.WriteFile(bindingPath, []byte(`[]`), 0600); err != nil {
				t.Fatal(err)
			}
			if detail = read(detailPath); detail.Item == nil || !detail.Capability.Allowed {
				t.Fatal("unexpected config reload", detail)
			}
			if _, err := db.Exec(`UPDATE channel_capabilities SET status='SUSPENDED'`); err != nil {
				t.Fatal(err)
			}
			if detail = read(detailPath); detail.Item != nil || detail.Capability.Allowed || detail.Capability.Reason != "SUSPENDED" {
				t.Fatal(detail)
			}
		})
	}
	var tracking, requests int
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&tracking); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM promotion_conversion_requests`).Scan(&requests); err != nil || tracking != 0 || requests != 0 {
		t.Fatal(tracking, requests, err)
	}
}

// Catches missing runtime wiring, bypassed signature/owner checks, invalid IDs,
// cross-owner cursors and metadata/authorization reads that cause writes.
func TestRuntimeMaterialsVerifiedIdentityPaginationAndIsolation(t *testing.T) {
	db, config, private, _ := runtimeMaterialFixture(t)
	h, err := NewHandler(context.Background(), db, config)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	defer server.Close()
	token := runtimeToken(t, private, runtimeMaterialOwner)
	base := "/api/v1/promoter/materials"
	got := getRuntimeMaterial(t, server, base+"?"+runtimeMaterialQuery+"&limit=1", token, 200)
	var page material.Page
	if err := json.Unmarshal(got.Data, &page); err != nil || len(page.Items) != 1 || page.Items[0].ID != runtimeMaterialID || page.Items[0].Title != "runtime title 1" || !page.Capability.Allowed || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	cursor := page.NextCursor
	got = getRuntimeMaterial(t, server, base+"?"+runtimeMaterialQuery+"&cursor="+url.QueryEscape(cursor)+"&limit=2", token, 200)
	page = material.Page{}
	if err := json.Unmarshal(got.Data, &page); err != nil || len(page.Items) != 1 || page.Items[0].ID != "00000000-0000-4000-8000-000000000002" || page.NextCursor != "" {
		t.Fatal(page, err)
	}
	got = getRuntimeMaterial(t, server, base+"/"+runtimeMaterialID+"?"+runtimeMaterialQuery, token, 200)
	var detail material.Detail
	if err := json.Unmarshal(got.Data, &detail); err != nil || detail.Item == nil || detail.Item.ID != runtimeMaterialID || !detail.Capability.Allowed || !detail.Availability.Available {
		t.Fatal(detail, err)
	}
	for _, path := range []string{base + "?" + runtimeMaterialQuery, base + "/" + runtimeMaterialID + "?" + runtimeMaterialQuery} {
		getRuntimeMaterial(t, server, path, "", 401)
		_, wrongKey := runtimeKeys(t)
		getRuntimeMaterial(t, server, path, runtimeToken(t, wrongKey, runtimeMaterialOwner), 401)
		got := getRuntimeMaterial(t, server, path, runtimeToken(t, private, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"), 200)
		if !strings.Contains(string(got.Data), `"reason":"POSITION_UNAVAILABLE"`) || strings.Contains(string(got.Data), `"item":`) || strings.Contains(string(got.Data), runtimeMaterialID) || (path == base+"?"+runtimeMaterialQuery && !strings.Contains(string(got.Data), `"items":[]`)) {
			t.Fatal(string(got.Data))
		}
	}
	getRuntimeMaterial(t, server, base+"?"+runtimeMaterialQuery+"&cursor="+url.QueryEscape(cursor), runtimeToken(t, private, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"), 400)
	for _, id := range []string{"bad-id", "00000000-0000-0000-0000-000000000000", "00000000-0000-4000-8000-00000000000A"} {
		getRuntimeMaterial(t, server, base+"/"+id+"?"+runtimeMaterialQuery, token, 400)
	}
	for _, extra := range []string{"&ownerId=other", "&mediaId=runtime-media", "&status=READY"} {
		getRuntimeMaterial(t, server, base+"?"+runtimeMaterialQuery+extra, token, 400)
	}
	for _, id := range []string{"00000000-0000-4000-8000-000000000099"} {
		got := getRuntimeMaterial(t, server, base+"/"+id+"?"+runtimeMaterialQuery, token, 200)
		if strings.Contains(string(got.Data), `"item"`) || !strings.Contains(string(got.Data), `"reason":"MATERIAL_UNAVAILABLE"`) {
			t.Fatal(string(got.Data))
		}
	}
	var tracks, requests, materials, proofs int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM tracking_records),(SELECT count(*) FROM promotion_conversion_requests),(SELECT count(*) FROM promotion_materials),(SELECT count(*) FROM channel_capability_evidence)`).Scan(&tracks, &requests, &materials, &proofs); err != nil || tracks != 0 || requests != 0 || materials != 2 || proofs != 1 {
		t.Fatal(tracks, requests, materials, proofs, err)
	}
}

// Catches stale grants, bypassed state/range filters or leaving a usable card
// after revocation/expiry in the fully wired HTTP -> verifier -> PG path.
func TestRuntimeMaterialsCurrentDenials(t *testing.T) {
	db, config, private, at := runtimeMaterialFixture(t)
	h, err := NewHandler(context.Background(), db, config)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	defer server.Close()
	token := runtimeToken(t, private, runtimeMaterialOwner)
	for _, tc := range []struct{ name, sql, capability, availability string }{
		{"capability", `UPDATE channel_capabilities SET status='SUSPENDED'`, "SUSPENDED", "CAPABILITY_UNAVAILABLE"},
		{"member", `UPDATE promoter_profiles SET status='DISABLED' WHERE application_id='runtime-app'`, "NOT_ENABLED", "CAPABILITY_UNAVAILABLE"},
		{"position", `UPDATE promotion_positions SET status='DISABLED'`, "POSITION_UNAVAILABLE", "CAPABILITY_UNAVAILABLE"},
		{"evidence expired", `INSERT INTO channel_capability_evidence(id,capability_id,owner_id,media_approval_ref,source_approval_ref,interface_version,real_call_evidence_ref,verified_at,expires_at,recorded_by) VALUES('f1111111-1111-4111-8111-111111111111','c1111111-1111-4111-8111-111111111111','operator','runtime-media-proof','runtime-source-proof','v1','runtime-call-proof',now()-interval '2 minutes',now()-interval '1 minute','runtime-test'); UPDATE channel_capabilities SET evidence_id='f1111111-1111-4111-8111-111111111111'`, "VERIFICATION_EXPIRED", "CAPABILITY_UNAVAILABLE"},
		{"material", `UPDATE promotion_materials SET status='SUSPENDED'`, "READY", "NOT_ACTIVE"},
		{"expired", `UPDATE promotion_materials SET ends_at=now()-interval '1 second'`, "READY", "EXPIRED"},
		{"region", `UPDATE promotion_materials SET region_mode='CITIES',city_codes=ARRAY['110100']`, "READY", "REGION_MISMATCH"},
		{"business", `UPDATE promotion_materials SET business='travel'`, "READY", "BUSINESS_MISMATCH"},
		{"terminal", `UPDATE promotion_materials SET terminals=ARRAY['WX_MINI']`, "READY", "TERMINAL_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(tc.sql); err != nil {
				t.Fatal(err)
			}
			got := getRuntimeMaterial(t, server, "/api/v1/promoter/materials?"+runtimeMaterialQuery, token, 200)
			var page material.Page
			if err := json.Unmarshal(got.Data, &page); err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" || page.Capability.Reason != tc.capability {
				t.Fatal(page, err)
			}
			got = getRuntimeMaterial(t, server, "/api/v1/promoter/materials/"+runtimeMaterialID+"?"+runtimeMaterialQuery, token, 200)
			var detail material.Detail
			if err := json.Unmarshal(got.Data, &detail); err != nil || detail.Item != nil || detail.Capability.Reason != tc.capability || detail.Availability.Reason != tc.availability || detail.Availability.Available {
				t.Fatal(detail, err)
			}
			if _, err := db.Exec(`UPDATE channel_capabilities SET status='READY',evidence_id='c2222222-2222-4222-8222-222222222222'; UPDATE promoter_profiles SET status='ENABLED' WHERE application_id='runtime-app'; UPDATE promotion_positions SET status='ENABLED'; UPDATE promotion_materials SET status='ACTIVE',region_mode='NATIONWIDE',city_codes=ARRAY[]::text[],business='',terminals=ARRAY['H5']`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE promotion_materials SET ends_at=$1`, at.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Catches startup accepting ambiguous configuration, media fallback, or leaks
// from configured storage failure while the absent-config branch stays closed.
func TestRuntimeMaterialsConfigurationAndStorageFailure(t *testing.T) {
	db, config, private, _ := runtimeMaterialFixture(t)
	token := runtimeToken(t, private, runtimeMaterialOwner)
	bad := config
	bad.CatalogBindings = append(append([]material.CatalogBinding{}, config.CatalogBindings...), config.CatalogBindings[0])
	if h, err := NewHandler(context.Background(), db, bad); err == nil || h != nil {
		t.Fatal("duplicate configuration accepted", err)
	}
	bad.CatalogBindings = []material.CatalogBinding{{Platform: "MT", Type: "PRODUCT", Terminal: "H5", Scene: "home", MediaID: "media"}}
	if h, err := NewHandler(context.Background(), db, bad); err == nil || h != nil {
		t.Fatal("unsupported config accepted", err)
	}
	for _, bindings := range [][]material.CatalogBinding{nil, {{Platform: "JD", Type: "PRODUCT", Terminal: "H5", Scene: "home", MediaID: "wrong-media"}}} {
		closed := config
		closed.CatalogBindings = bindings
		h, err := NewHandler(context.Background(), db, closed)
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(h)
		for _, path := range []string{"/api/v1/promoter/materials?" + runtimeMaterialQuery, "/api/v1/promoter/materials/" + runtimeMaterialID + "?" + runtimeMaterialQuery} {
			got := getRuntimeMaterial(t, server, path, token, 200)
			if !strings.Contains(string(got.Data), `"reason":"UNCONFIGURED"`) || strings.Contains(string(got.Data), `"item":`) || strings.Contains(string(got.Data), runtimeMaterialID) || (path == "/api/v1/promoter/materials?"+runtimeMaterialQuery && !strings.Contains(string(got.Data), `"items":[]`)) {
				t.Fatal(string(got.Data))
			}
		}
		server.Close()
	}
	h, err := NewHandler(context.Background(), db, config)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(h)
	defer server.Close()
	if _, err := db.Exec(`DROP TABLE promotion_materials`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/promoter/materials?" + runtimeMaterialQuery, "/api/v1/promoter/materials/" + runtimeMaterialID + "?" + runtimeMaterialQuery} {
		got := getRuntimeMaterial(t, server, path, token, 503)
		if got.Code != "MATERIALS_UNAVAILABLE" {
			t.Fatal(got.Code)
		}
	}
}
