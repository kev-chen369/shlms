package coupon

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid coupon catalog query")

type Item struct {
	ID             string    `json:"id"`
	Platform       string    `json:"platform"`
	ClaimMode      string    `json:"claimMode"`
	ActionLabel    string    `json:"actionLabel"`
	Title          string    `json:"title"`
	Scope          string    `json:"scope"`
	Currency       string    `json:"currency"`
	DiscountMinor  int64     `json:"discountMinor"`
	ThresholdMinor int64     `json:"thresholdMinor"`
	CityCode       string    `json:"cityCode"`
	Business       string    `json:"business"`
	RuleVersion    string    `json:"ruleVersion"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor"`
}

type ListInput struct {
	Platform string
	Cursor   string
	Limit    int
}

type Catalog struct{ DB *sql.DB }

func action(mode string) string {
	switch mode {
	case "IN_SITE_VERIFIED":
		return "立即领取"
	case "PLATFORM_CLAIM":
		return "前往平台领券"
	case "BUNDLED_OFFER":
		return "领券购买"
	case "PLATFORM_ACTIVITY":
		return "去平台领取 / 购买"
	default:
		return ""
	}
}

// List returns only explicitly enabled, verified and currently valid materials.
// This read path never manufactures an upstream coupon or claims a consumer benefit.
func (c Catalog) List(ctx context.Context, in ListInput) (Page, error) {
	if (in.Platform != "" && in.Platform != "JD" && in.Platform != "TB" && in.Platform != "MT") || in.Limit < 1 || in.Limit > 100 || c.DB == nil {
		return Page{}, ErrInvalid
	}
	after := ""
	if in.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		if err != nil || len(raw) > 160 {
			return Page{}, ErrInvalid
		}
		parts := strings.SplitN(string(raw), "\x00", 2)
		if len(parts) != 2 || parts[0] != in.Platform || len(parts[1]) == 0 || len(parts[1]) > 128 || strings.TrimSpace(parts[1]) != parts[1] {
			return Page{}, ErrInvalid
		}
		after = parts[1]
	}
	rows, err := c.DB.QueryContext(ctx, `SELECT id,platform,claim_mode,title,scope,currency,discount_minor,threshold_minor,city_code,business,rule_version,updated_at,expires_at
        FROM coupon_catalog WHERE enabled AND verified_at IS NOT NULL AND verified_at <= CURRENT_TIMESTAMP
        AND updated_at <= CURRENT_TIMESTAMP AND expires_at > CURRENT_TIMESTAMP AND ($1 = '' OR platform = $1)
        AND id > $2 ORDER BY id LIMIT $3`, in.Platform, after, in.Limit+1)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	page := Page{Items: []Item{}}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Platform, &item.ClaimMode, &item.Title, &item.Scope, &item.Currency,
			&item.DiscountMinor, &item.ThresholdMinor, &item.CityCode, &item.Business, &item.RuleVersion, &item.UpdatedAt, &item.ExpiresAt); err != nil {
			return Page{}, err
		}
		item.ActionLabel = action(item.ClaimMode)
		if item.ActionLabel == "" {
			return Page{}, errors.New("unknown coupon mode")
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	if len(page.Items) > in.Limit {
		page.Items = page.Items[:in.Limit]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(in.Platform + "\x00" + page.Items[len(page.Items)-1].ID))
	}
	return page, nil
}
