package material

import (
	stdcontext "context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/kev-chen369/shlms/internal/capability"
)

// Detail is a safe metadata read, not permission to generate a promotion link.
// A denial never carries an item or internal source/evidence data.
type Detail struct {
	Item         *Card               `json:"item,omitempty"`
	Capability   capability.Decision `json:"capability"`
	Availability Decision            `json:"availability"`
}

// Get uses trusted auth/config inputs and the same read-only snapshot for all
// gates. Future generation must independently recheck grants in its transaction.
func (r Repository) Get(ctx stdcontext.Context, ownerID string, scope Context, key capability.Key, materialID string, now time.Time) (Detail, error) {
	q := Query{OwnerID: ownerID, Scope: scope, Limit: 1}
	if !validQuery(q) || !validID(materialID) || now.IsZero() || key.Kind != "CATALOG" || key.Platform != scope.Platform || key.MaterialType != scope.Type || key.CityCode != scope.CityCode || key.Business != scope.Business || key.Terminal != scope.Terminal || !text(key.MediaID, 128, false) || !text(key.PositionID, 128, false) || !text(key.Scene, 80, false) {
		return Detail{}, ErrInvalid
	}
	if r.DB == nil {
		return Detail{}, ErrUnavailable
	}
	fail := func() (Detail, error) {
		if ctx.Err() != nil {
			return Detail{}, ctx.Err()
		}
		return Detail{}, ErrUnavailable
	}
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return fail()
	}
	defer tx.Rollback()
	decision, err := capability.CheckInTransaction(ctx, tx, ownerID, key, now)
	if err != nil {
		return fail()
	}
	detail := Detail{Capability: decision, Availability: Decision{Reason: "CAPABILITY_UNAVAILABLE"}}
	if decision.Allowed {
		var record Record
		var starts sql.NullTime
		var cities, terminals string
		err := tx.QueryRowContext(ctx, `SELECT id::text,platform,material_type,external_material_id,canonical_url,title,status,starts_at,ends_at,source_updated_at,rule_version,evidence_ref,region_mode,array_to_json(city_codes)::text,business,array_to_json(terminals)::text
 FROM promotion_materials WHERE id=$1::uuid AND platform=$2 AND material_type=$3
 AND EXISTS(SELECT 1 FROM promoter_profiles WHERE user_id=$4 AND status='ENABLED')`, materialID, scope.Platform, scope.Type, ownerID).
			Scan(&record.ID, &record.Platform, &record.Type, &record.ExternalMaterialID, &record.CanonicalURL, &record.Title, &record.Status, &starts, &record.EndsAt, &record.SourceUpdatedAt, &record.RuleVersion, &record.EvidenceRef, &record.Region.Mode, &cities, &record.Business, &terminals)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			detail.Availability = Decision{Reason: "MATERIAL_UNAVAILABLE"}
		case err != nil:
			return fail()
		default:
			if json.Unmarshal([]byte(cities), &record.Region.CityCodes) != nil || json.Unmarshal([]byte(terminals), &record.Terminals) != nil {
				return fail()
			}
			if starts.Valid {
				record.StartsAt = starts.Time
			}
			card, available := CardFor(record, scope, now)
			detail.Availability = available
			if available.Available {
				detail.Item = &card
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fail()
	}
	return detail, nil
}
