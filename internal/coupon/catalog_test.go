package coupon

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kev-chen369/shlms/internal/dbmigrate"
)

func testDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("coupon_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = admin.Close() })
	if _, err := dbmigrate.Run(context.Background(), db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCatalogOnlyListsVerifiedCurrentMaterial(t *testing.T) {
	db := testDB(t)
	catalog := Catalog{DB: db}
	page, err := catalog.List(context.Background(), ListInput{Limit: 20})
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	insert := `INSERT INTO coupon_catalog(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
        VALUES($1,$2,'PLATFORM_CLAIM','真实渠道活动','PRODUCT',100,1000,'v1','review-1',now(),now()-interval '1 hour',$3,$4)`
	for _, row := range []struct {
		id, platform string
		expiry       time.Time
		enabled      bool
	}{
		{"a", "JD", time.Now().Add(time.Hour), true},
		{"b", "JD", time.Now().Add(time.Hour), true},
		{"c", "TB", time.Now().Add(time.Hour), true},
		{"d", "JD", time.Now().Add(-time.Minute), true},
		{"e", "JD", time.Now().Add(time.Hour), false},
	} {
		if _, err := db.Exec(insert, row.id, row.platform, row.expiry, row.enabled); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO coupon_products(coupon_id,external_product_id,title,enabled,verified_at,updated_at,expires_at)
			VALUES($1,'sku-' || $1,'已核验商品',true,now(),now()-interval '1 hour',now()+interval '1 day')`, row.id); err != nil {
			t.Fatal(err)
		}
	}
	first, err := catalog.List(context.Background(), ListInput{Platform: "JD", Limit: 1})
	if err != nil || len(first.Items) != 1 || first.Items[0].ID != "a" || first.Items[0].ActionLabel != "前往平台领券" || first.NextCursor == "" {
		t.Fatal(first, err)
	}
	second, err := catalog.List(context.Background(), ListInput{Platform: "JD", Limit: 1, Cursor: first.NextCursor})
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != "b" || second.NextCursor != "" {
		t.Fatal(second, err)
	}
	if _, err := catalog.List(context.Background(), ListInput{Platform: "TB", Limit: 1, Cursor: first.NextCursor}); err != ErrInvalid {
		t.Fatal("cross-platform cursor accepted", err)
	}
}

func TestCouponDetailAndProductScope(t *testing.T) {
	db := testDB(t)
	c := Catalog{DB: db}
	_, err := db.Exec(`INSERT INTO coupon_catalog
		(id,platform,claim_mode,title,scope,scope_external_id,scope_name,discount_minor,threshold_minor,city_code,business,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
		VALUES ('city-coupon','MT','PLATFORM_ACTIVITY','城市活动','SHOP','shop-1','门店一',500,3000,'110100','food','v1','review-1',now(),now()-interval '1 hour',now()+interval '1 day',true),
		('expired-coupon','JD','BUNDLED_OFFER','过期活动','PRODUCT','','',500,3000,'','','v1','review-2',now()-interval '2 days',now()-interval '3 days',now()-interval '1 day',true),
		('unmapped-coupon','JD','BUNDLED_OFFER','无商品活动','PRODUCT','','',500,3000,'','','v1','review-3',now(),now()-interval '1 hour',now()+interval '1 day',true),
		('shop-without-id','MT','PLATFORM_ACTIVITY','无店铺活动','SHOP','','',500,3000,'','','v1','review-4',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id      string
		enabled bool
		expires string
	}{
		{"p1", true, "1 day"}, {"p2", true, "1 day"}, {"p3", false, "1 day"}, {"p4", true, "-30 minutes"},
	} {
		_, err = db.Exec(`INSERT INTO coupon_products(coupon_id,external_product_id,title,enabled,verified_at,updated_at,expires_at)
			VALUES('city-coupon',$1,'真实商品',$2,now(),now()-interval '1 hour',now()+($3::interval))`, row.id, row.enabled, row.expires)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Get(context.Background(), "city-coupon", "", "food"); err != ErrNotFound {
		t.Fatal("city-limited coupon leaked", err)
	}
	if _, err := c.Get(context.Background(), "city-coupon", "120100", "food"); err != ErrNotFound {
		t.Fatal("wrong city accepted", err)
	}
	if _, err := c.Get(context.Background(), "city-coupon", "110100", "hotel"); err != ErrNotFound {
		t.Fatal("wrong business accepted", err)
	}
	item, err := c.Get(context.Background(), "city-coupon", "110100", "food")
	if err != nil || item.ScopeExternalID != "shop-1" || item.ScopeName != "门店一" {
		t.Fatal(item, err)
	}
	if _, err := c.Get(context.Background(), "expired-coupon", "", ""); err != ErrNotFound {
		t.Fatal("expired coupon visible", err)
	}
	if _, err := c.Get(context.Background(), "unmapped-coupon", "", ""); err != ErrNotFound {
		t.Fatal("unmapped product coupon visible", err)
	}
	if _, err := c.Get(context.Background(), "shop-without-id", "", ""); err != ErrNotFound {
		t.Fatal("unmapped shop coupon visible", err)
	}
	page, err := c.List(context.Background(), ListInput{Platform: "MT", Limit: 20})
	if err != nil || len(page.Items) != 0 {
		t.Fatal("city-limited coupon leaked in list", page, err)
	}
	first, err := c.Products(context.Background(), "city-coupon", "110100", "food", "", 1)
	if err != nil || len(first.Items) != 1 || first.Items[0].ExternalProductID != "p1" || first.NextCursor == "" {
		t.Fatal(first, err)
	}
	second, err := c.Products(context.Background(), "city-coupon", "110100", "food", first.NextCursor, 1)
	if err != nil || len(second.Items) != 1 || second.Items[0].ExternalProductID != "p2" || second.NextCursor != "" {
		t.Fatal(second, err)
	}
	if _, err := c.Products(context.Background(), "city-coupon", "120100", "food", "", 1); err != ErrNotFound {
		t.Fatal("wrong-city products visible", err)
	}
	if _, err := c.Products(context.Background(), "city-coupon", "110100", "food", first.NextCursor, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Products(context.Background(), "city-coupon", "110100", "hotel", first.NextCursor, 1); err != ErrNotFound {
		t.Fatal("wrong business accepted", err)
	}
}
