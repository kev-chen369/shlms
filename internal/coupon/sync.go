package coupon

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrStaleEvidence = errors.New("coupon evidence is stale or conflicting")

// VerifiedSnapshot must come from an approved channel verifier. EvidenceRef is an
// opaque audit reference, never a channel URL, token or raw response body.
type VerifiedSnapshot struct {
	Item        Item
	Products    []Product
	EvidenceRef string
	VerifiedAt  time.Time
	Active      bool
}

type Verifier interface {
	Verify(context.Context, string) (VerifiedSnapshot, error)
}

// Synchronizer is deliberately absent from production HTTP wiring until a real
// channel verifier is installed. Callers cannot publish a client supplied coupon.
type Synchronizer struct {
	DB       *sql.DB
	Verifier Verifier
}

func (s Synchronizer) Sync(ctx context.Context, id string) error {
	if s.DB == nil || s.Verifier == nil || !validID(id) {
		return ErrInvalid
	}
	snapshot, err := s.Verifier.Verify(ctx, id)
	if err != nil {
		return err
	}
	snapshot.VerifiedAt = snapshot.VerifiedAt.UTC().Truncate(time.Microsecond)
	snapshot.Item.UpdatedAt = snapshot.Item.UpdatedAt.UTC().Truncate(time.Microsecond)
	snapshot.Item.ExpiresAt = snapshot.Item.ExpiresAt.UTC().Truncate(time.Microsecond)
	for i := range snapshot.Products {
		snapshot.Products[i].UpdatedAt = snapshot.Products[i].UpdatedAt.UTC().Truncate(time.Microsecond)
		snapshot.Products[i].ExpiresAt = snapshot.Products[i].ExpiresAt.UTC().Truncate(time.Microsecond)
	}
	if !validSnapshot(id, snapshot) {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serializes first insert and later updates for this logical coupon.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(9217, hashtext($1))`, id); err != nil {
		return err
	}
	var oldPlatform, oldEvidence string
	var oldVerified time.Time
	var oldEnabled bool
	err = tx.QueryRowContext(ctx, `SELECT platform,evidence_ref,verified_at,enabled FROM coupon_catalog WHERE id=$1 FOR UPDATE`, id).
		Scan(&oldPlatform, &oldEvidence, &oldVerified, &oldEnabled)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if exists {
		if oldPlatform != snapshot.Item.Platform || snapshot.VerifiedAt.Before(oldVerified) {
			return ErrStaleEvidence
		}
		if snapshot.VerifiedAt.Equal(oldVerified) {
			if oldEvidence == snapshot.EvidenceRef && oldEnabled == snapshot.Active {
				return tx.Commit()
			}
			return ErrStaleEvidence
		}
	}
	if !snapshot.Active {
		if !exists {
			return ErrNotFound
		}
		if _, err = tx.ExecContext(ctx, `UPDATE coupon_catalog SET enabled=false,verified_at=$2,evidence_ref=$3 WHERE id=$1`,
			id, snapshot.VerifiedAt, snapshot.EvidenceRef); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE coupon_products SET enabled=false,verified_at=$2 WHERE coupon_id=$1`, id, snapshot.VerifiedAt); err != nil {
			return err
		}
	} else {
		item := snapshot.Item
		_, err = tx.ExecContext(ctx, `INSERT INTO coupon_catalog
			(id,platform,claim_mode,title,scope,scope_external_id,scope_name,currency,discount_minor,threshold_minor,
			city_code,city_name,business,rule_version,evidence_ref,verified_at,updated_at,expires_at,enabled)
			VALUES($1,$2,$3,$4,$5,$6,$7,'CNY',$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,true)
			ON CONFLICT(id) DO UPDATE SET claim_mode=EXCLUDED.claim_mode,title=EXCLUDED.title,scope=EXCLUDED.scope,
			scope_external_id=EXCLUDED.scope_external_id,scope_name=EXCLUDED.scope_name,
			discount_minor=EXCLUDED.discount_minor,threshold_minor=EXCLUDED.threshold_minor,city_code=EXCLUDED.city_code,city_name=EXCLUDED.city_name,
			business=EXCLUDED.business,rule_version=EXCLUDED.rule_version,evidence_ref=EXCLUDED.evidence_ref,
			verified_at=EXCLUDED.verified_at,updated_at=EXCLUDED.updated_at,expires_at=EXCLUDED.expires_at,enabled=true`,
			id, item.Platform, item.ClaimMode, item.Title, item.Scope, item.ScopeExternalID, item.ScopeName,
			item.DiscountMinor, item.ThresholdMinor, item.CityCode, item.CityName, item.Business, item.RuleVersion,
			snapshot.EvidenceRef, snapshot.VerifiedAt, item.UpdatedAt, item.ExpiresAt)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM coupon_products WHERE coupon_id=$1`, id); err != nil {
			return err
		}
		for _, p := range snapshot.Products {
			_, err = tx.ExecContext(ctx, `INSERT INTO coupon_products
				(coupon_id,external_product_id,title,enabled,verified_at,updated_at,expires_at)
				VALUES($1,$2,$3,true,$4,$5,$6)`, id, p.ExternalProductID, p.Title,
				snapshot.VerifiedAt, p.UpdatedAt, p.ExpiresAt)
			if err != nil {
				return err
			}
		}
	}
	action := "PUBLISH"
	if !snapshot.Active {
		action = "REVOKE"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO coupon_sync_events(coupon_id,evidence_ref,verified_at,action) VALUES($1,$2,$3,$4)`,
		id, snapshot.EvidenceRef, snapshot.VerifiedAt, action); err != nil {
		return err
	}
	return tx.Commit()
}

func validSnapshot(id string, s VerifiedSnapshot) bool {
	if s.Item.ID != id || (s.Item.Platform != "JD" && s.Item.Platform != "TB" && s.Item.Platform != "MT") ||
		!strings.HasPrefix(id, s.Item.Platform+":") || len(id) <= len(s.Item.Platform)+1 ||
		!validText(s.EvidenceRef, 256) ||
		strings.Contains(s.EvidenceRef, "://") || s.VerifiedAt.IsZero() || s.VerifiedAt.After(time.Now().UTC()) {
		return false
	}
	if !s.Active {
		return true
	}
	i := s.Item
	if action(i.ClaimMode) == "" || (i.Scope != "PRODUCT" && i.Scope != "CATEGORY" && i.Scope != "SHOP" && i.Scope != "ACTIVITY") ||
		!validText(i.Title, 256) || !validText(i.RuleVersion, 80) || !validContext(i.CityCode, i.Business) ||
		i.DiscountMinor < 0 || i.ThresholdMinor < 0 || i.Currency != "CNY" || i.UpdatedAt.IsZero() ||
		i.UpdatedAt.After(s.VerifiedAt) || !i.ExpiresAt.After(s.VerifiedAt) || len(s.Products) > 500 {
		return false
	}
	if (i.Scope == "SHOP" || i.Scope == "CATEGORY") && (!validText(i.ScopeExternalID, 128) || !validText(i.ScopeName, 256)) {
		return false
	}
	if (i.CityCode == "" && i.CityName != "") || (i.CityCode != "" && !validText(i.CityName, 80)) {
		return false
	}
	if i.Scope == "PRODUCT" && len(s.Products) == 0 {
		return false
	}
	seen := make(map[string]bool, len(s.Products))
	for _, p := range s.Products {
		if !validID(p.ExternalProductID) || !validText(p.Title, 256) || p.UpdatedAt.IsZero() ||
			p.UpdatedAt.After(s.VerifiedAt) || !p.ExpiresAt.After(s.VerifiedAt) || seen[p.ExternalProductID] {
			return false
		}
		seen[p.ExternalProductID] = true
	}
	return true
}

func validText(value string, max int) bool {
	return len(value) > 0 && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
