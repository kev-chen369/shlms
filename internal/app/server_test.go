package app

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

func runtimeDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("app_runtime_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	return db
}

func runtimeKeys(t *testing.T) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), key
}

func runtimeToken(t *testing.T, key *rsa.PrivateKey, subject string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "at+jwt"})
	claims, _ := json.Marshal(map[string]any{"iss": "https://issuer.example", "aud": "shlms-api", "sub": subject, "exp": time.Now().Add(10 * time.Minute).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "permissions": []string{"promoter:review"}})
	enc := base64.RawURLEncoding.EncodeToString
	signed := enc(header) + "." + enc(claims)
	hash := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + enc(signature)
}

func TestRuntimeRequiresConfigAndAppliedMigrations(t *testing.T) {
	db := runtimeDB(t)
	public, _ := runtimeKeys(t)
	config := Config{PublicKeyPEM: public, Issuer: "https://issuer.example", Audience: "shlms-api", AgreementVersion: "v1", MigrationsDir: "../../migrations"}
	if _, err := NewHandler(context.Background(), db, Config{}); err == nil {
		t.Fatal("empty config accepted")
	}
	if _, err := NewHandler(context.Background(), db, config); err == nil {
		t.Fatal("missing schema accepted")
	}
	if _, err := dbmigrate.Run(context.Background(), db, config.MigrationsDir); err != nil {
		t.Fatal(err)
	}
	withoutIssuer := config
	withoutIssuer.Issuer = ""
	if _, err := NewHandler(context.Background(), db, withoutIssuer); err == nil {
		t.Fatal("missing issuer accepted")
	}
	withoutAgreement := config
	withoutAgreement.AgreementVersion = ""
	if _, err := NewHandler(context.Background(), db, withoutAgreement); err == nil {
		t.Fatal("missing agreement version accepted")
	}
	if _, err := NewHandler(context.Background(), db, config); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeWiresVerifiedIdentityAndDatabasePermissions(t *testing.T) {
	db := runtimeDB(t)
	public, private := runtimeKeys(t)
	config := Config{PublicKeyPEM: public, Issuer: "https://issuer.example", Audience: "shlms-api", AgreementVersion: "v1", MigrationsDir: "../../migrations"}
	if _, err := dbmigrate.Run(context.Background(), db, config.MigrationsDir); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(context.Background(), db, config)
	if err != nil {
		t.Fatal(err)
	}
	userToken := runtimeToken(t, private, "user-1")
	adminToken := runtimeToken(t, private, "admin-1")
	call := func(method, path, body, token, key string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s = %d: %s", method, path, w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	call("GET", "/healthz", "", "", "", 200)
	call("GET", "/api/v1/promoter/profile", "", "", "", 401)
	profile := call("GET", "/api/v1/promoter/profile", "", userToken, "", 200)
	if profile["data"].(map[string]any)["status"] != "NOT_APPLIED" {
		t.Fatal(profile)
	}
	application := call("POST", "/api/v1/promoter/applications", `{"displayName":"Alice","scene":"group","agreementVersion":"v1","agreed":true}`, userToken, "apply-1", 200)
	appID := application["data"].(map[string]any)["applicationId"].(string)
	reviewPath := "/admin/v1/promoter-applications/" + appID + "/review"
	reviewBody := `{"approve":true,"reason":"approved","version":1}`
	// The forged permissions claim in the signed token has no administrative effect.
	call("POST", reviewPath, reviewBody, adminToken, "review-1", 403)
	if _, err := db.Exec(`INSERT INTO admin_principals(user_id,active) VALUES('admin-1',true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO admin_permissions(user_id,permission) VALUES('admin-1','promoter:review')`); err != nil {
		t.Fatal(err)
	}
	approved := call("POST", reviewPath, reviewBody, adminToken, "review-1", 200)
	if approved["data"].(map[string]any)["status"] != "ENABLED" {
		t.Fatal(approved)
	}
	position := call("POST", "/api/v1/promotion-positions", `{"name":"group","scene":"sharing"}`, userToken, "position-1", 200)
	if position["data"].(map[string]any)["canConvert"] != false {
		t.Fatal(position)
	}
	if _, err := db.Exec(`UPDATE admin_principals SET active=false WHERE user_id='admin-1'`); err != nil {
		t.Fatal(err)
	}
	call("GET", "/admin/v1/promoter-applications", "", adminToken, "", 403)
}
