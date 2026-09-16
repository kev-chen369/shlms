package conversion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func conversionDB(t *testing.T) *sql.DB {
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
	schema := fmt.Sprintf("conversion_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	for _, name := range []string{"000001_tracking_records", "000002_promoter_applications", "000004_promotion_positions", "000007_promotion_previews", "000008_promotion_conversion_requests"} {
		b, err := os.ReadFile("../../migrations/" + name + ".up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range []string{"u1", "u2"} {
		if _, err = db.Exec(`INSERT INTO promoter_profiles(user_id,status) VALUES($1,'NOT_APPLIED')`, user); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO promoter_applications(id,user_id,idempotency_key,display_name,scene,agreement_version,consented_at,status)
			VALUES($1,$2,'apply','User','home','v1',CURRENT_TIMESTAMP,'ENABLED')`, "app-"+user, user); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`UPDATE promoter_profiles SET status='ENABLED',application_id=$1,version=1 WHERE user_id=$2`, "app-"+user, user); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,version) VALUES($1,$2,'main','home','ENABLED',1)`, "pos-"+user, user); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO promotion_previews(id,owner_user_id,position_id,idempotency_key,request_fingerprint,scene,channel,external_product_id,product_name,currency,coupon_price_minor,promoter_estimate_minor,consumer_cashback_estimate_minor,rule_version,evidence_ref,updated_at,expires_at)
			VALUES($1,$2,$3,'preview','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','home','JD','sku','product','CNY',1000,100,50,'v1','evidence',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP + interval '10 minutes')`, "pv-"+user, user, "pos-"+user); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func request() ReserveInput {
	return ReserveInput{OwnerUserID: "u1", PositionID: "pos-u1", PreviewID: "pv-u1", IdempotencyKey: "convert-1",
		RequestFingerprint: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Scene: "home"}
}

func TestReserveConcurrencyAndOwnerIsolation(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const n = 12
	results := make([]Record, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = repo.Reserve(ctx, request()) }(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i].ID != results[0].ID || results[i].TrackingID != results[0].TrackingID {
			t.Fatalf("caller %d: %+v %v", i, results[i], errs[i])
		}
	}
	if results[0].Status != "PENDING" {
		t.Fatal(results[0])
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM promotion_conversion_requests`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	changed := request()
	changed.RequestFingerprint = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if _, err := repo.Reserve(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(ctx, "u2", results[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if got, err := repo.FindByID(ctx, "u1", results[0].ID); err != nil || got.PreviewID != "pv-u1" {
		t.Fatal(got, err)
	}
	other := request()
	other.OwnerUserID, other.PositionID, other.PreviewID = "u2", "pos-u2", "pv-u2"
	if _, err := repo.Reserve(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}

func TestReserveRejectsWrongOwnerExpiredDisabledAndRollsBack(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx := context.Background()
	wrong := request()
	wrong.PreviewID = "pv-u2"
	if _, err := repo.Reserve(ctx, wrong); !errors.Is(err, ErrPreview) {
		t.Fatal(err)
	}
	wrong = request()
	wrong.Scene = "other"
	if _, err := repo.Reserve(ctx, wrong); !errors.Is(err, ErrPreview) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promotion_previews SET expires_at=CURRENT_TIMESTAMP - interval '1 minute',updated_at=CURRENT_TIMESTAMP - interval '2 minutes' WHERE id='pv-u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reserve(ctx, request()); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promotion_previews SET expires_at=CURRENT_TIMESTAMP + interval '10 minutes' WHERE id='pv-u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='DISABLED' WHERE user_id='u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reserve(ctx, request()); !errors.Is(err, ErrNotEnabled) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='ENABLED' WHERE user_id='u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE FUNCTION reject_conversion() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject insert'; END; $$;
		CREATE TRIGGER reject_conversion BEFORE INSERT ON promotion_conversion_requests FOR EACH ROW EXECUTE FUNCTION reject_conversion()`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Reserve(ctx, request()); err == nil {
		t.Fatal("injected failure accepted")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tracking_records`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestReserveInputValidation(t *testing.T) {
	r := request()
	if !r.valid() {
		t.Fatal(r)
	}
	r.RequestFingerprint = "invalid"
	if r.valid() {
		t.Fatal(r)
	}
}

func TestOwnerCanReadHistoricStatusAfterDisable(t *testing.T) {
	db := conversionDB(t)
	repo := Repository{DB: db}
	ctx := context.Background()
	created, err := repo.Reserve(ctx, request())
	if err != nil {
		t.Fatal(err)
	}
	reader := ReadService{Repository: repo}
	if _, err := reader.Get(ctx, "u2", created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE promoter_profiles SET status='DISABLED' WHERE user_id='u1'`); err != nil {
		t.Fatal(err)
	}
	got, err := reader.Get(ctx, "u1", created.ID)
	if err != nil || got.ID != created.ID || got.Status != "PENDING" || got.LinkURL != "" || got.SchemeURL != "" {
		t.Fatal(got, err)
	}
	if _, err := db.Exec(`UPDATE promotion_conversion_requests SET status='SUCCEEDED',link_url='https://channel.example/item',scheme_url='jd://item',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, created.ID); err != nil {
		t.Fatal(err)
	}
	got, err = reader.Get(ctx, "u1", created.ID)
	if err != nil || got.Status != "SUCCEEDED" || got.LinkURL != "https://channel.example/item" || got.SchemeURL != "jd://item" {
		t.Fatal(got, err)
	}
	if _, err := reader.Get(ctx, "u1", " invalid-id "); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
