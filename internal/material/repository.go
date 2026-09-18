package material

import (
	stdcontext "context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/kev-chen369/shlms/internal/capability"
)

var ErrUnavailable = errors.New("material storage unavailable")

// Repository reads trusted metadata only. It does not import or fetch URLs.
type Repository struct{ DB *sql.DB }
type Page struct {
	Items      []Card              `json:"items"`
	NextCursor string              `json:"nextCursor,omitempty"`
	Capability capability.Decision `json:"capability"`
}

// List binds a trusted owner and catalog declaration to one read-only snapshot.
// This is not a generation lock; future generation must recheck all grants.
func (r Repository) List(ctx stdcontext.Context, q Query, key capability.Key, now time.Time) (Page, error) {
	after, err := ParseQuery(q)
	if err != nil || now.IsZero() || key.Kind != "CATALOG" || key.Platform != q.Scope.Platform || key.MaterialType != q.Scope.Type || key.CityCode != q.Scope.CityCode || key.Business != q.Scope.Business || key.Terminal != q.Scope.Terminal || !text(key.MediaID, 128, false) || !text(key.PositionID, 128, false) || !text(key.Scene, 80, false) {
		return Page{}, ErrInvalid
	}
	if r.DB == nil {
		return Page{}, ErrUnavailable
	}
	fail := func() (Page, error) {
		if ctx.Err() != nil {
			return Page{}, ctx.Err()
		}
		return Page{}, ErrUnavailable
	}
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return fail()
	}
	defer tx.Rollback()
	decision, err := capability.CheckInTransaction(ctx, tx, q.OwnerID, key, now)
	if err != nil {
		return fail()
	}
	page := Page{Items: make([]Card, 0, q.Limit), Capability: decision}
	if !decision.Allowed {
		if err := tx.Commit(); err != nil {
			return fail()
		}
		return page, nil
	}
	for {
		// Cast a sentinel UUID, never an empty string. Each query independently
		// binds owner eligibility and all public filters within the same snapshot.
		boundary := after
		if boundary == "" {
			boundary = "00000000-0000-0000-0000-000000000000"
		}
		rows, err := tx.QueryContext(ctx, `SELECT id::text,platform,material_type,external_material_id,canonical_url,title,status,starts_at,ends_at,source_updated_at,rule_version,evidence_ref,region_mode,array_to_json(city_codes)::text,business,array_to_json(terminals)::text
 FROM promotion_materials
 WHERE EXISTS(SELECT 1 FROM promoter_profiles WHERE user_id=$1 AND status='ENABLED')
 AND platform=$2 AND material_type=$3 AND status='ACTIVE' AND id>$4::uuid
 AND (starts_at IS NULL OR starts_at<=$5) AND ends_at>$5 AND source_updated_at<=$5
 AND (region_mode='NATIONWIDE' OR (region_mode='CITIES' AND $6=ANY(city_codes)))
 AND (business='' OR business=$7) AND $8=ANY(terminals)
 ORDER BY id LIMIT $9`, q.OwnerID, q.Scope.Platform, q.Scope.Type, boundary, now, q.Scope.CityCode, q.Scope.Business, q.Scope.Terminal, q.Limit+1)
		if err != nil {
			return fail()
		}
		count := 0
		for rows.Next() {
			var record Record
			var starts sql.NullTime
			var cities, terminals string
			err := rows.Scan(&record.ID, &record.Platform, &record.Type, &record.ExternalMaterialID, &record.CanonicalURL, &record.Title, &record.Status, &starts, &record.EndsAt, &record.SourceUpdatedAt, &record.RuleVersion, &record.EvidenceRef, &record.Region.Mode, &cities, &record.Business, &terminals)
			if err != nil || json.Unmarshal([]byte(cities), &record.Region.CityCodes) != nil || json.Unmarshal([]byte(terminals), &record.Terminals) != nil {
				_ = rows.Close()
				return fail()
			}
			if starts.Valid {
				record.StartsAt = starts.Time
			}
			after = record.ID
			count++
			if card, available := CardFor(record, q.Scope, now); available.Available {
				page.Items = append(page.Items, card)
			}
			if len(page.Items) > q.Limit {
				break
			}
		}
		rowErr := rows.Err()
		closeErr := rows.Close()
		if rowErr != nil || closeErr != nil {
			return fail()
		}
		if len(page.Items) > q.Limit {
			page.Items = page.Items[:q.Limit]
			page.NextCursor, err = EncodeCursor(q, page.Items[q.Limit-1].ID)
			if err != nil {
				return Page{}, ErrInvalid
			}
			break
		}
		if count < q.Limit+1 {
			break
		}
	}
	if err := tx.Commit(); err != nil {
		return fail()
	}
	return page, nil
}
