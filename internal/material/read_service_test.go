package material

import (
	stdcontext "context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kev-chen369/shlms/internal/capability"
)

// Catches client-controlled media, retaining the caller's configuration slice,
// or bypassing current capability checks for either list or detail reads.
func TestReadServiceTrustedBindingAndRevocation(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	insertCatalogMaterial(t, db, 1, fixture())
	insertCatalogMaterial(t, db, 2, fixture())
	bindings := []CatalogBinding{{Platform: "JD", Type: "PRODUCT", Terminal: "H5", Scene: "home", MediaID: "catalog-media"}}
	s, err := NewReadService(Repository{DB: db}, bindings)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	bindings[0].MediaID = "attacker-media"
	in := ReadInput{OwnerID: q.OwnerID, Scope: q.Scope, PositionID: key.PositionID, Scene: "home"}
	ctx := stdcontext.Background()
	page, err := s.List(ctx, in, "", 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != catalogID(1) || page.NextCursor == "" || !page.Capability.Allowed {
		t.Fatal(page, err)
	}
	continued, err := s.List(ctx, in, page.NextCursor, 2)
	if err != nil || len(continued.Items) != 1 || continued.Items[0].ID != catalogID(2) || continued.NextCursor != "" || !continued.Capability.Allowed {
		t.Fatal(continued, err)
	}
	detail, err := s.Get(ctx, in, catalogID(2))
	if err != nil || detail.Item == nil || detail.Item.ID != catalogID(2) || !detail.Availability.Available || !detail.Capability.Allowed {
		t.Fatal(detail, err)
	}
	if _, err := db.Exec(`UPDATE channel_capabilities SET status='SUSPENDED'`); err != nil {
		t.Fatal(err)
	}
	page, err = s.List(ctx, in, page.NextCursor, 1)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" || page.Capability.Allowed || page.Capability.Reason != "SUSPENDED" {
		t.Fatal(page, err)
	}
	detail, err = s.Get(ctx, in, catalogID(2))
	if err != nil || detail.Item != nil || detail.Capability.Reason != "SUSPENDED" || detail.Availability.Available || detail.Availability.Reason != "CAPABILITY_UNAVAILABLE" {
		t.Fatal(detail, err)
	}
	in.OwnerID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	page, err = s.List(ctx, in, "", 1)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.Capability.Reason != "POSITION_UNAVAILABLE" {
		t.Fatal(page, err)
	}
}

// Catches fallback/wildcard bindings and inventing material reads without config.
func TestReadServiceMissingAndExactConfiguration(t *testing.T) {
	db, q, key := catalogDB(t)
	readyCatalog(t, db)
	insertCatalogMaterial(t, db, 1, fixture())
	s, err := NewReadService(Repository{DB: db}, []CatalogBinding{{"JD", "PRODUCT", "H5", "home", "other-media"}})
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	in := ReadInput{OwnerID: q.OwnerID, Scope: q.Scope, PositionID: key.PositionID, Scene: "home"}
	// Correct shape with the wrong trusted media must not reuse the READY grant.
	page, err := s.List(stdcontext.Background(), in, "", 1)
	if err != nil || page.Capability.Allowed || page.Capability.Reason != "UNCONFIGURED" || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
	if _, err := db.Exec(`DROP TABLE promotion_materials`); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ReadInput){func(in *ReadInput) { in.Scope.Platform = "TB" }, func(in *ReadInput) { in.Scope.Type = "ACTIVITY" }, func(in *ReadInput) { in.Scope.Terminal = "WX_MINI" }, func(in *ReadInput) { in.Scene = "other" }} {
		requested := in
		change(&requested)
		page, err := s.List(stdcontext.Background(), requested, "", 1)
		if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != "" || page.Capability != (capability.Decision{Reason: "UNCONFIGURED"}) {
			t.Fatal(page, err)
		}
		detail, err := s.Get(stdcontext.Background(), requested, catalogID(1))
		if err != nil || detail.Item != nil || detail.Capability != (capability.Decision{Reason: "UNCONFIGURED"}) || detail.Availability != (Decision{Reason: "CAPABILITY_UNAVAILABLE"}) {
			t.Fatal(detail, err)
		}
	}
	empty, err := NewReadService(Repository{DB: db}, nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err = empty.List(stdcontext.Background(), in, "", 1)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.Capability.Reason != "UNCONFIGURED" {
		t.Fatal(page, err)
	}
}

// Catches ambiguous configs, malformed input accepted before the missing-config
// branch, or turning nil service/storage and cancellation into a successful read.
func TestReadServiceInvalidConfigurationAndInputs(t *testing.T) {
	db, q, key := catalogDB(t)
	valid := CatalogBinding{"JD", "PRODUCT", "H5", "home", "catalog-media"}
	for _, bad := range []CatalogBinding{{"MT", "PRODUCT", "H5", "home", "media"}, {"JD", "PRODUCT", "web", "home", "media"}, {"JD", "PRODUCT", "H5", "", "media"}, {"JD", "PRODUCT", "H5", "home", ""}, {"JD", "PRODUCT", "H5", "home", "\u00a0media"}} {
		if s, err := NewReadService(Repository{DB: db}, []CatalogBinding{bad}); !errors.Is(err, ErrInvalid) || s != nil {
			t.Fatal(s, err)
		}
	}
	if s, err := NewReadService(Repository{DB: db}, []CatalogBinding{valid, valid}); !errors.Is(err, ErrInvalid) || s != nil {
		t.Fatal(s, err)
	}
	if s, err := NewReadService(Repository{}, nil); !errors.Is(err, ErrUnavailable) || s != nil {
		t.Fatal(s, err)
	}
	s, err := NewReadService(Repository{DB: db}, nil)
	if err != nil {
		t.Fatal(err)
	}
	in := ReadInput{OwnerID: q.OwnerID, Scope: q.Scope, PositionID: key.PositionID, Scene: "home"}
	for _, change := range []func(*ReadInput){func(in *ReadInput) { in.OwnerID = "owner" }, func(in *ReadInput) { in.Scope.Terminal = "WEB" }, func(in *ReadInput) { in.Scope.CityCode = " bad" }, func(in *ReadInput) { in.Scene = "" }, func(in *ReadInput) { in.PositionID = "" }} {
		bad := in
		change(&bad)
		page, err := s.List(stdcontext.Background(), bad, "", 1)
		if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(page, Page{}) {
			t.Fatal(page, err)
		}
		detail, err := s.Get(stdcontext.Background(), bad, catalogID(1))
		if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(detail, Detail{}) {
			t.Fatal(detail, err)
		}
	}
	for _, args := range []struct {
		cursor string
		limit  int
	}{{"invalid", 1}, {"", 0}, {"", 101}} {
		page, err := s.List(stdcontext.Background(), in, args.cursor, args.limit)
		if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(page, Page{}) {
			t.Fatal(page, err)
		}
	}
	detail, err := s.Get(stdcontext.Background(), in, "bad-id")
	if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(detail, Detail{}) {
		t.Fatal(detail, err)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	page, err := s.List(ctx, in, "", 1)
	if !errors.Is(err, stdcontext.Canceled) || !reflect.DeepEqual(page, Page{}) {
		t.Fatal(page, err)
	}
	detail, err = s.Get(ctx, in, catalogID(1))
	if !errors.Is(err, stdcontext.Canceled) || !reflect.DeepEqual(detail, Detail{}) {
		t.Fatal(detail, err)
	}
	var absent *ReadService
	page, err = absent.List(stdcontext.Background(), in, "", 1)
	if err != ErrUnavailable || !reflect.DeepEqual(page, Page{}) {
		t.Fatal(page, err)
	}
	detail, err = absent.Get(stdcontext.Background(), in, catalogID(1))
	if err != ErrUnavailable || !reflect.DeepEqual(detail, Detail{}) {
		t.Fatal(detail, err)
	}
}
