package dashboard

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

func dashboardDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN is not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*cfg)
	schema := fmt.Sprintf("dashboard_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = admin.Close() })
	if _, err = dbmigrate.Run(context.Background(), db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCountsOwnRecordsAndDates(t *testing.T) {
	db := dashboardDB(t)
	ctx := context.Background()
	for _, owner := range []string{"u1", "u2"} {
		stmts := []struct {
			q    string
			args []any
		}{
			{`INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED')`, []any{owner}},
			{`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES($1,$2,'main','home','ENABLED',1)`, []any{"pos-" + owner, owner}},
			{`INSERT INTO promotion_previews(id,owner_user_id,position_id,idempotency_key,request_fingerprint,scene,channel,external_product_id,product_name,currency,coupon_price_minor,promoter_estimate_minor,consumer_cashback_estimate_minor,rule_version,evidence_ref,updated_at,expires_at) VALUES($1,$2,$3,'preview','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','home','JD','sku','product','CNY',100,10,0,'v1','evidence',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP + interval '1 day')`, []any{"pv-" + owner, owner, "pos-" + owner}},
			{`INSERT INTO tracking_records(id,idempotency_key,user_id,channel,external_product_id,source,created_at) VALUES($1,$2,$3,'JD','sku','PROMOTION_CENTER',CURRENT_TIMESTAMP)`, []any{"tr-" + owner, "key-" + owner, owner}},
			{`INSERT INTO promotion_conversion_requests(id,owner_user_id,position_id,preview_id,tracking_id,idempotency_key,request_fingerprint,scene,status,channel_request_id,link_url,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'convert','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','home','SUCCEEDED',$1,'https://approved.example/item',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, []any{"cr-" + owner, owner, "pos-" + owner, "pv-" + owner, "tr-" + owner}},
			{`INSERT INTO promotion_share_events(owner_user_id,event_id,conversion_request_id,action,artifact_type,scene) VALUES($1,'event-1',$2,'COPY_REPORTED','link','home')`, []any{owner, "cr-" + owner}},
		}
		for _, s := range stmts {
			if _, err := db.Exec(s.q, s.args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, row := range []struct{ id, owner, status string }{{"paid", "u1", "PAID"}, {"refunded", "u1", "REFUNDED"}, {"other", "u2", "PAID"}} {
		_, err := db.Exec(`INSERT INTO order_raw_events(id,channel,event_id,external_order_id,event_type,occurred_at,payload_sha256,payload_nonce,payload_ciphertext,key_version)
   VALUES($1,'JD',$1,$1,'ORDER',CURRENT_TIMESTAMP,decode(repeat('aa',32),'hex'),decode(repeat('bb',12),'hex'),decode(repeat('cc',17),'hex'),'test')`, row.id)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO normalized_orders(channel,external_order_id,status,status_at,latest_evidence_id,order_occurred_at)
   VALUES('JD',$1,$2,CURRENT_TIMESTAMP,$1,CURRENT_TIMESTAMP)`, row.id, row.status)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO order_attributions(channel,external_order_id,evidence_id,status,method,evidence_value_hash,owner_user_id,position_id,tracking_id)
   VALUES('JD',$1,$1,'ATTRIBUTED','SUB_ID',repeat('a',64),$2,$3,$4)`, row.id, row.owner, "pos-"+row.owner, "tr-"+row.owner)
		if err != nil {
			t.Fatal(err)
		}
	}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(zone)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	filter := Filter{OwnerUserID: "u1", From: from, To: from.AddDate(0, 0, 1)}
	got, err := (ReadStore{DB: db}).Get(ctx, filter)
	if err != nil || got.SuccessfulLinks != 1 || got.CopyReports != 1 || got.ValidOrders != 1 || got.TimeZone != "Asia/Shanghai" || got.AsOf.IsZero() {
		t.Fatal(got, err)
	}
	filter.Channel = "MT"
	got, err = (ReadStore{DB: db}).Get(ctx, filter)
	if err != nil || got.SuccessfulLinks != 0 || got.CopyReports != 0 {
		t.Fatal(got, err)
	}
	filter.Channel = ""
	filter.PositionID = "pos-u2"
	got, err = (ReadStore{DB: db}).Get(ctx, filter)
	if err != nil || got.SuccessfulLinks != 0 || got.CopyReports != 0 {
		t.Fatal(got, err)
	}
	filter.PositionID = ""
	filter.From = from.AddDate(0, 0, -1)
	filter.To = from
	got, err = (ReadStore{DB: db}).Get(ctx, filter)
	if err != nil || got.SuccessfulLinks != 0 || got.CopyReports != 0 {
		t.Fatal(got, err)
	}
}

func TestInvalidFilter(t *testing.T) {
	if (Filter{}).Valid() {
		t.Fatal("empty filter accepted")
	}
	now := time.Now()
	if (Filter{OwnerUserID: "u1", From: now, To: now.Add(367 * 24 * time.Hour)}).Valid() {
		t.Fatal("unbounded range accepted")
	}
}
