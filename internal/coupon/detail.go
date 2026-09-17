package coupon

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

type Product struct {
	ExternalProductID string    `json:"externalProductId"`
	Title             string    `json:"title"`
	UpdatedAt         time.Time `json:"updatedAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ProductPage struct {
	Items      []Product `json:"items"`
	NextCursor string    `json:"nextCursor"`
}

func validID(id string) bool {
	return len(id) > 0 && len(id) <= 128 && strings.TrimSpace(id) == id && !strings.ContainsAny(id, "\x00\r\n")
}

// Get never returns a disabled, expired or geographically inapplicable coupon.
func (c Catalog) Get(ctx context.Context, id, city, business string) (Item, error) {
	if c.DB == nil || !validID(id) || !validContext(city, business) {
		return Item{}, ErrInvalid
	}
	var item Item
	err := c.DB.QueryRowContext(ctx, `SELECT id,platform,claim_mode,title,scope,scope_external_id,scope_name,currency,
		discount_minor,threshold_minor,city_code,city_name,business,rule_version,updated_at,expires_at
		FROM coupon_catalog c WHERE id=$1 AND enabled AND verified_at <= CURRENT_TIMESTAMP
		AND updated_at <= CURRENT_TIMESTAMP AND expires_at > CURRENT_TIMESTAMP
		AND (city_code='' OR city_code=$2) AND (business='' OR business=$3)
		AND (scope NOT IN ('SHOP','CATEGORY') OR scope_external_id <> '')
		AND (scope <> 'PRODUCT' OR EXISTS (SELECT 1 FROM coupon_products p WHERE p.coupon_id = c.id
			AND p.enabled AND p.verified_at <= CURRENT_TIMESTAMP AND p.updated_at <= CURRENT_TIMESTAMP AND p.expires_at > CURRENT_TIMESTAMP))`, id, city, business).Scan(
		&item.ID, &item.Platform, &item.ClaimMode, &item.Title, &item.Scope, &item.ScopeExternalID, &item.ScopeName,
		&item.Currency, &item.DiscountMinor, &item.ThresholdMinor, &item.CityCode, &item.CityName, &item.Business,
		&item.RuleVersion, &item.UpdatedAt, &item.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	item.ActionLabel = action(item.ClaimMode)
	if item.ActionLabel == "" {
		return Item{}, errors.New("unknown coupon mode")
	}
	return item, nil
}

// Products lists only independently verified and still active product relations.
func (c Catalog) Products(ctx context.Context, id, city, business, cursor string, limit int) (ProductPage, error) {
	if limit < 1 || limit > 100 || !validID(id) || !validContext(city, business) {
		return ProductPage{}, ErrInvalid
	}
	item, err := c.Get(ctx, id, city, business)
	if err != nil {
		return ProductPage{}, err
	}
	if item.Scope != "PRODUCT" && item.Scope != "CATEGORY" && item.Scope != "SHOP" {
		return ProductPage{Items: []Product{}}, nil
	}
	after := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || len(raw) > 335 {
			return ProductPage{}, ErrInvalid
		}
		parts := strings.Split(string(raw), "\x00")
		if len(parts) != 4 || parts[0] != id || parts[1] != city || parts[2] != business || !validID(parts[3]) {
			return ProductPage{}, ErrInvalid
		}
		after = parts[3]
	}
	rows, err := c.DB.QueryContext(ctx, `SELECT external_product_id,title,updated_at,expires_at
		FROM coupon_products WHERE coupon_id=$1 AND enabled AND verified_at <= CURRENT_TIMESTAMP
		AND updated_at <= CURRENT_TIMESTAMP AND expires_at > CURRENT_TIMESTAMP
		AND external_product_id > $2 ORDER BY external_product_id LIMIT $3`, id, after, limit+1)
	if err != nil {
		return ProductPage{}, err
	}
	defer rows.Close()
	page := ProductPage{Items: []Product{}}
	for rows.Next() {
		var product Product
		if err := rows.Scan(&product.ExternalProductID, &product.Title, &product.UpdatedAt, &product.ExpiresAt); err != nil {
			return ProductPage{}, err
		}
		page.Items = append(page.Items, product)
	}
	if err := rows.Err(); err != nil {
		return ProductPage{}, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(id + "\x00" + city + "\x00" + business + "\x00" + page.Items[len(page.Items)-1].ExternalProductID))
	}
	return page, nil
}
