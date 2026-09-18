package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/material"
)

// Catches ignoring the deployment env, losing other startup fields or fallback.
func TestLoadAPIConfig(t *testing.T) {
	t.Setenv("AUTH_ISSUER", "https://test-issuer.example")
	t.Setenv("AUTH_AUDIENCE", "test-audience")
	t.Setenv("PROMOTER_AGREEMENT_VERSION", "test-v1")
	path := filepath.Join(t.TempDir(), "bindings.json")
	if err := os.WriteFile(path, []byte(`[{"platform":"TB","type":"ACTIVITY","terminal":"WX_MINI","scene":"event","mediaId":"synthetic-tb"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CATALOG_BINDINGS_FILE", path)
	config, err := loadAPIConfig([]byte("test-public-key"))
	if err != nil || config.Issuer != "https://test-issuer.example" || config.Audience != "test-audience" || config.AgreementVersion != "test-v1" || config.MigrationsDir != "migrations" || string(config.PublicKeyPEM) != "test-public-key" || !reflect.DeepEqual(config.CatalogBindings, []material.CatalogBinding{{Platform: "TB", Type: "ACTIVITY", Terminal: "WX_MINI", Scene: "event", MediaID: "synthetic-tb"}}) {
		t.Fatal(config, err)
	}
	t.Setenv("CATALOG_BINDINGS_FILE", "")
	config, err = loadAPIConfig(nil)
	if err != nil || len(config.CatalogBindings) != 0 {
		t.Fatal(config, err)
	}
	t.Setenv("CATALOG_BINDINGS_FILE", path+"-missing")
	config, err = loadAPIConfig(nil)
	if !errors.Is(err, material.ErrInvalid) || config.Issuer != "" || config.CatalogBindings != nil {
		t.Fatal(config, err)
	}
}

// Executes the real entrypoint; removing its loader would reach DATABASE_URL
// handling instead of the sanitized configuration failure, never a listener.
func TestMainRejectsBrokenCatalogConfig(t *testing.T) {
	if os.Getenv("SHLMS_TEST_MAIN_CHILD") == "1" {
		main()
		return
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(key, []byte("synthetic-key"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHLMS_TEST_MAIN_CHILD", "1")
	t.Setenv("DATABASE_URL", "not-a-database-url")
	t.Setenv("AUTH_PUBLIC_KEY_FILE", key)
	t.Setenv("CATALOG_BINDINGS_FILE", filepath.Join(dir, "private-missing.json"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMainRejectsBrokenCatalogConfig$").CombinedOutput()
	if ctx.Err() != nil || err == nil || !strings.Contains(string(raw), "catalog binding configuration is invalid or unreadable") {
		t.Fatal(string(raw), err, ctx.Err())
	}
	for _, private := range []string{dir, "not-a-database-url", "synthetic-key", "api listening"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("startup leak or fallback", string(raw))
		}
	}
}
