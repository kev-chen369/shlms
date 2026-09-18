package conversion

import (
	"context"
	"database/sql"
	"errors"
	"net/url"

	"github.com/kev-chen369/shlms/internal/linkresolve"
)

var ErrShareNotReady = errors.New("share link is not ready")

type ShareArtifact struct {
	RequestID string `json:"requestId"`
	Type      string `json:"type"`
	Content   string `json:"content"`
}

type ShareReader struct{ DB *sql.DB }

// GetArtifact exposes only a stored successful public link and product title.
// Preview reward estimates and channel evidence are never included.
func (s ShareReader) GetArtifact(ctx context.Context, owner, requestID, kind string) (ShareArtifact, error) {
	if s.DB == nil || !validText(owner, 128) || !validText(requestID, 128) || (kind != "link" && kind != "text") {
		return ShareArtifact{}, ErrInvalid
	}
	var status, link, title string
	err := s.DB.QueryRowContext(ctx, `SELECT r.status,COALESCE(r.link_url,''),p.product_name
		FROM promotion_conversion_requests r JOIN promotion_previews p ON p.id=r.preview_id
		WHERE r.id=$1 AND r.owner_user_id=$2`, requestID, owner).Scan(&status, &link, &title)
	if errors.Is(err, sql.ErrNoRows) {
		return ShareArtifact{}, ErrNotFound
	}
	if err != nil {
		return ShareArtifact{}, err
	}
	if status != "SUCCEEDED" {
		return ShareArtifact{}, ErrShareNotReady
	}
	u, err := url.Parse(link)
	if err != nil || u.Hostname() == "" {
		return ShareArtifact{}, ErrShareNotReady
	}
	policy, err := linkresolve.NewPolicy([]string{u.Hostname()})
	if err != nil {
		return ShareArtifact{}, ErrShareNotReady
	}
	if _, err := policy.Validate(link); err != nil {
		return ShareArtifact{}, ErrShareNotReady
	}
	content := link
	if kind == "text" {
		content = title + "\n" + link + "\n价格及优惠以平台结算页为准"
	}
	return ShareArtifact{RequestID: requestID, Type: kind, Content: content}, nil
}
