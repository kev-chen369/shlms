package coupon

import (
	"context"
	"testing"
)

func TestCitiesOnlyExposeVerifiedUsableMaterial(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO coupon_catalog
		(id,platform,claim_mode,title,scope,discount_minor,threshold_minor,city_code,city_name,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
		VALUES
		('MT:bj','MT','PLATFORM_ACTIVITY','北京活动','ACTIVITY',100,1000,'110100','北京','v1','e1',now(),now()-interval '1 hour',now()+interval '1 day',true),
		('MT:sh-expired','MT','PLATFORM_ACTIVITY','上海活动','ACTIVITY',100,1000,'310100','上海','v1','e2',now()-interval '2 day',now()-interval '2 day',now()-interval '1 day',true),
		('MT:global','MT','PLATFORM_ACTIVITY','全国活动','ACTIVITY',100,1000,'','','v1','e3',now(),now()-interval '1 hour',now()+interval '1 day',true)`)
	if err != nil {
		t.Fatal(err)
	}
	c := Catalog{DB: db}
	cities, err := c.Cities(context.Background(), "MT")
	if err != nil || len(cities) != 1 || cities[0].Code != "110100" || cities[0].Name != "北京" {
		t.Fatal(cities, err)
	}
	global, err := c.List(context.Background(), ListInput{Platform: "MT", Limit: 20})
	if err != nil || len(global.Items) != 1 || global.Items[0].ID != "MT:global" {
		t.Fatal(global, err)
	}
	beijing, err := c.List(context.Background(), ListInput{Platform: "MT", CityCode: "110100", Limit: 20})
	if err != nil || len(beijing.Items) != 2 {
		t.Fatal(beijing, err)
	}
	item, err := c.Get(context.Background(), "MT:bj", "110100", "")
	if err != nil || item.CityName != "北京" {
		t.Fatal(item, err)
	}
	if _, err := c.Get(context.Background(), "MT:bj", "310100", ""); err != ErrNotFound {
		t.Fatal(err)
	}
	if _, err := c.Cities(context.Background(), "PDD"); err != ErrInvalid {
		t.Fatal(err)
	}
}
