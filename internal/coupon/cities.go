package coupon

import "context"

type City struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Cities only exposes named cities with currently verified, usable material.
// An empty result means the UI can still show nationwide material.
func (c Catalog) Cities(ctx context.Context, platform string) ([]City, error) {
	if c.DB == nil || (platform != "JD" && platform != "TB" && platform != "MT") {
		return nil, ErrInvalid
	}
	rows, err := c.DB.QueryContext(ctx, `SELECT DISTINCT ON (c.city_code) c.city_code,c.city_name
		FROM coupon_catalog c WHERE c.platform=$1 AND c.city_code<>'' AND c.city_name<>''
		AND c.enabled AND c.verified_at<=CURRENT_TIMESTAMP AND c.updated_at<=CURRENT_TIMESTAMP AND c.expires_at>CURRENT_TIMESTAMP
		AND (c.scope NOT IN ('SHOP','CATEGORY') OR c.scope_external_id<>'')
		AND (c.scope<>'PRODUCT' OR EXISTS (SELECT 1 FROM coupon_products p WHERE p.coupon_id=c.id
			AND p.enabled AND p.verified_at<=CURRENT_TIMESTAMP AND p.updated_at<=CURRENT_TIMESTAMP AND p.expires_at>CURRENT_TIMESTAMP))
		ORDER BY c.city_code,c.updated_at DESC`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []City{}
	for rows.Next() {
		var city City
		if err := rows.Scan(&city.Code, &city.Name); err != nil {
			return nil, err
		}
		result = append(result, city)
	}
	return result, rows.Err()
}
