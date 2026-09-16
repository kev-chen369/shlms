package promoter

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type listRepoFunc func(context.Context, ApplicationListQuery) ([]ApplicationListItem, error)

func (f listRepoFunc) ListCurrentApplications(ctx context.Context, q ApplicationListQuery) ([]ApplicationListItem, error) {
	return f(ctx, q)
}
func listActor() AdminActor {
	return AdminActor{ID: "admin", Permissions: map[string]bool{ReadPermission: true}}
}

func TestAdminListValidation(t *testing.T) {
	s := AdminListService{Repository: listRepoFunc(func(context.Context, ApplicationListQuery) ([]ApplicationListItem, error) {
		t.Fatal("invalid request reached database")
		return nil, nil
	})}
	if _, err := s.ListApplications(context.Background(), adminActor(), ApplicationListInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	for _, in := range []ApplicationListInput{{Status: "INVALID"}, {Limit: -1}, {Limit: 101}, {Cursor: "invalid!"}} {
		if _, err := s.ListApplications(context.Background(), listActor(), in); !errors.Is(err, ErrInvalidInput) {
			t.Fatal(in, err)
		}
	}
}

func TestPostgresAdminListPagination(t *testing.T) {
	db := promoterDB(t)
	repo := NewPostgresRepository(db)
	s := AdminListService{Repository: repo}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for i := 0; i < 5; i++ {
		a := application()
		a.ID = fmt.Sprintf("app-%d", i)
		a.UserID = fmt.Sprintf("user-%d", i)
		if _, err := repo.Submit(ctx, "key", a); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 3; i++ {
		page, err := s.ListApplications(ctx, listActor(), ApplicationListInput{Status: Pending, Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ApplicationID] || item.Version != 1 || item.Status != Pending {
				t.Fatal(item)
			}
			seen[item.ApplicationID] = true
		}
		if i == 0 {
			if page.Items[0].ApplicationID != "app-4" {
				t.Fatal("unstable tie order", page)
			}
			if _, err := s.ListApplications(ctx, listActor(), ApplicationListInput{Status: Rejected, Cursor: page.NextCursor}); !errors.Is(err, ErrInvalidInput) {
				t.Fatal("filter mismatch", err)
			}
		}
		cursor = page.NextCursor
	}
	if len(seen) != 5 || cursor != "" {
		t.Fatal(seen, cursor)
	}
	page, err := s.ListApplications(ctx, listActor(), ApplicationListInput{Status: Rejected})
	if err != nil || len(page.Items) != 0 || page.Items == nil || page.NextCursor != "" {
		t.Fatal(page, err)
	}
}
