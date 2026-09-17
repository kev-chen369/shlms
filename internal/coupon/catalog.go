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
var ErrNotFound = errors.New("coupon not found")

type Item struct {
	ID              string    `json:"id"`
	Platform        string    `json:"platform"`
	ClaimMode       string    `json:"claimMode"`
	ActionLabel     string    `json:"actionLabel"`
	Title           string    `json:"title"`
	Scope           string    `json:"scope"`
	ScopeExternalID string    `json:"scopeExternalId"`
	ScopeName       string    `json:"scopeName"`
	Currency        string    `json:"currency"`
	DiscountMinor   int64     `json:"discountMinor"`
	ThresholdMinor  int64     `json:"thresholdMinor"`
	CityCode        string    `json:"cityCode"`
	Business        string    `json:"business"`
	RuleVersion     string    `json:"ruleVersion"`
	UpdatedAt       time.Time `json:"updatedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor"`
}

type ListInput struct {
	Platform string
	CityCode string
	Business string
	Cursor   string
	Limit    int
}

func validContext(city, business string) bool {
	return len(city) <= 32 && len(business) <= 40 &&
		strings.TrimSpace(city) == city && strings.TrimSpace(business) == business &&
		!strings.ContainsAny(city+business, "\x00\r\n")
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
	if (in.Platform != "" && in.Platform != "JD" && in.Platform != "TB" && in.Platform != "MT") || !validContext(in.CityCode, in.Business) || in.Limit < 1 || in.Limit > 100 || c.DB == nil {
		return Page{}, ErrInvalid
	}
	after := ""
	if in.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		if err != nil || len(raw) > 210 {
			return Page{}, ErrInvalid
		}
		parts := strings.Split(string(raw), "\x00")
		if len(parts) != 4 || parts[0] != in.Platform || parts[1] != in.CityCode || parts[2] != in.Business || len(parts[3]) == 0 || len(parts[3]) > 128 || strings.TrimSpace(parts[3]) != parts[3] {
			return Page{}, ErrInvalid
		}
		after = parts[3]
	}
	rows, err := c.DB.QueryContext(ctx, `SELECT id,platform,claim_mode,title,scope,scope_external_id,scope_name,currency,discount_minor,threshold_minor,city_code,business,rule_version,updated_at,expires_at
		FROM coupon_catalog c WHERE enabled AND verified_at IS NOT NULL AND verified_at <= CURRENT_TIMESTAMP
        AND updated_at <= CURRENT_TIMESTAMP AND expires_at > CURRENT_TIMESTAMP AND ($1 = '' OR platform = $1)
		AND (city_code = '' OR city_code = $2) AND (business = '' OR business = $3)
		AND (scope NOT IN ('SHOP','CATEGORY') OR scope_external_id <> '')
		AND (scope <> 'PRODUCT' OR EXISTS (SELECT 1 FROM coupon_products p WHERE p.coupon_id = c.id
			AND p.enabled AND p.verified_at <= CURRENT_TIMESTAMP AND p.updated_at <= CURRENT_TIMESTAMP AND p.expires_at > CURRENT_TIMESTAMP))
        AND id > $4 ORDER BY id LIMIT $5`, in.Platform, in.CityCode, in.Business, after, in.Limit+1)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	page := Page{Items: []Item{}}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Platform, &item.ClaimMode, &item.Title, &item.Scope, &item.ScopeExternalID, &item.ScopeName, &item.Currency,
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
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(in.Platform + "\x00" + in.CityCode + "\x00" + in.Business + "\x00" + page.Items[len(page.Items)-1].ID))
	}
	return page, nil
}
