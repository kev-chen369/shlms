package promoter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const ReadPermission = "promoter:read"

type ApplicationListInput struct {
	Status Status
	Limit  int
	Cursor string
}
type ApplicationListItem struct {
	ApplicationID string    `json:"applicationId"`
	UserID        string    `json:"userId"`
	DisplayName   string    `json:"displayName"`
	Scene         string    `json:"scene"`
	ConsentedAt   time.Time `json:"consentedAt"`
	Status        Status    `json:"status"`
	Version       int64     `json:"version"`
}
type ApplicationPage struct {
	Items      []ApplicationListItem `json:"items"`
	NextCursor string                `json:"nextCursor"`
}
type ApplicationListQuery struct {
	Status     Status
	Limit      int
	BeforeTime time.Time
	BeforeID   string
}
type ApplicationListRepository interface {
	ListCurrentApplications(context.Context, ApplicationListQuery) ([]ApplicationListItem, error)
}
type AdminListService struct{ Repository ApplicationListRepository }
type applicationCursor struct {
	Status Status    `json:"status"`
	Time   time.Time `json:"time"`
	ID     string    `json:"id"`
}

func (s AdminListService) ListApplications(ctx context.Context, actor AdminActor, in ApplicationListInput) (ApplicationPage, error) {
	if strings.TrimSpace(actor.ID) == "" || !actor.Permissions[ReadPermission] {
		return ApplicationPage{}, ErrForbidden
	}
	switch in.Status {
	case "", Pending, Enabled, Rejected, Disabled:
	default:
		return ApplicationPage{}, ErrInvalidInput
	}
	if in.Limit == 0 {
		in.Limit = 20
	}
	if in.Limit < 1 || in.Limit > 100 || len(in.Cursor) > 2048 {
		return ApplicationPage{}, ErrInvalidInput
	}
	q := ApplicationListQuery{Status: in.Status, Limit: in.Limit + 1}
	if in.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		if err != nil {
			return ApplicationPage{}, ErrInvalidInput
		}
		var c applicationCursor
		if err = json.Unmarshal(b, &c); err != nil || c.Status != in.Status || c.Time.IsZero() || !validText(c.ID, 256) {
			return ApplicationPage{}, ErrInvalidInput
		}
		q.BeforeTime, q.BeforeID = c.Time, c.ID
	}
	if s.Repository == nil {
		return ApplicationPage{}, errors.New("admin list repository unavailable")
	}
	items, err := s.Repository.ListCurrentApplications(ctx, q)
	if err != nil {
		return ApplicationPage{}, err
	}
	page := ApplicationPage{Items: items}
	if page.Items == nil {
		page.Items = []ApplicationListItem{}
	}
	if len(items) > in.Limit {
		page.Items = items[:in.Limit]
		last := page.Items[len(page.Items)-1]
		b, err := json.Marshal(applicationCursor{Status: in.Status, Time: last.ConsentedAt, ID: last.ApplicationID})
		if err != nil {
			return ApplicationPage{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	return page, nil
}

func (r PostgresRepository) ListCurrentApplications(ctx context.Context, q ApplicationListQuery) ([]ApplicationListItem, error) {
	if q.Limit < 1 || q.Limit > 101 {
		return nil, ErrInvalidInput
	}
	var before any
	if !q.BeforeTime.IsZero() {
		before = q.BeforeTime
	}
	rows, err := r.db.QueryContext(ctx, `SELECT a.id,a.user_id,a.display_name,a.scene,a.consented_at,p.status,p.version
		FROM promoter_profiles p JOIN promoter_applications a ON a.id=p.application_id AND a.user_id=p.user_id
		WHERE ($1='' OR p.status=$1) AND ($2::timestamptz IS NULL OR (a.consented_at,a.id)<($2::timestamptz,$3))
		ORDER BY a.consented_at DESC,a.id DESC LIMIT $4`, q.Status, before, q.BeforeID, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ApplicationListItem{}
	for rows.Next() {
		var item ApplicationListItem
		if err = rows.Scan(&item.ApplicationID, &item.UserID, &item.DisplayName, &item.Scene, &item.ConsentedAt, &item.Status, &item.Version); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
