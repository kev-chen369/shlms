package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/promoter"
)

func adminTestDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("admin_auth_%d", time.Now().UnixNano())
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
	b, err := os.ReadFile("../../migrations/000006_admin_authorizations.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(b)); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAdminResolverUsesDatabasePermissionsOnly(t *testing.T) {
	db := adminTestDB(t)
	verifier, key := tokenFixture(t)
	claims := goodClaims()
	claims["role"] = "superadmin"
	claims["permissions"] = []string{promoter.DisablePermission}
	token := signedToken(t, key, goodHeader(), claims)
	request := httptest.NewRequest("GET", "/admin/v1/promoter-applications", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	resolver := AdminResolver{Verifier: verifier, Store: PostgresAdminStore{DB: db}}
	actor, err := resolver.ResolveAdmin(request)
	if err != nil || actor.ID != "user-1" || len(actor.Permissions) != 0 {
		t.Fatal(actor, err)
	}
	if _, err = db.Exec(`INSERT INTO admin_principals(user_id,active) VALUES('user-1',true)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO admin_permissions(user_id,permission) VALUES('user-1',$1)`, promoter.ReviewPermission); err != nil {
		t.Fatal(err)
	}
	actor, err = resolver.ResolveAdmin(request)
	if err != nil || !actor.Permissions[promoter.ReviewPermission] || actor.Permissions[promoter.DisablePermission] {
		t.Fatal(actor, err)
	}
	if _, err = db.Exec(`UPDATE admin_principals SET active=false WHERE user_id='user-1'`); err != nil {
		t.Fatal(err)
	}
	actor, err = resolver.ResolveAdmin(request)
	if err != nil || len(actor.Permissions) != 0 {
		t.Fatal("disabled admin retained rights", actor, err)
	}
	if _, err = db.Exec(`DELETE FROM admin_permissions WHERE user_id='user-1'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE admin_principals SET active=true WHERE user_id='user-1'`); err != nil {
		t.Fatal(err)
	}
	actor, err = resolver.ResolveAdmin(request)
	if err != nil || len(actor.Permissions) != 0 {
		t.Fatal("revoked permission retained", actor, err)
	}
}

type permissionStoreFunc func(context.Context, string) (map[string]bool, error)

func (f permissionStoreFunc) PermissionsForUser(ctx context.Context, id string) (map[string]bool, error) {
	return f(ctx, id)
}

func TestAdminResolverFailsClosed(t *testing.T) {
	verifier, key := tokenFixture(t)
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+signedToken(t, key, goodHeader(), goodClaims()))
	if _, err := (AdminResolver{Verifier: verifier}).ResolveAdmin(request); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	failing := AdminResolver{Verifier: verifier, Store: permissionStoreFunc(func(context.Context, string) (map[string]bool, error) { return nil, errors.New("database unreachable") })}
	if _, err := failing.ResolveAdmin(request); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	request.Header.Del("Authorization")
	if _, err := failing.ResolveAdmin(request); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}

func TestPermissionStoreRespondsToConcurrentRevocation(t *testing.T) {
	db := adminTestDB(t)
	store := PostgresAdminStore{DB: db}
	if _, err := db.Exec(`INSERT INTO admin_principals(user_id,active) VALUES('admin',true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO admin_permissions(user_id,permission) VALUES('admin',$1)`, promoter.ReadPermission); err != nil {
		t.Fatal(err)
	}
	const readers = 8
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rights, err := store.PermissionsForUser(context.Background(), "admin")
			if err != nil || !rights[promoter.ReadPermission] {
				t.Error(rights, err)
			}
		}()
	}
	wg.Wait()
	if _, err := db.Exec(`UPDATE admin_principals SET active=false WHERE user_id='admin'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < readers; i++ {
		rights, err := store.PermissionsForUser(context.Background(), "admin")
		if err != nil || len(rights) != 0 {
			t.Fatal(rights, err)
		}
	}
}

var _ interface {
	ResolveAdmin(*http.Request) (promoter.AdminActor, error)
} = AdminResolver{}
