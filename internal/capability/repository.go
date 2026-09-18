package capability

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid capability query")
	ErrUnavailable = errors.New("capability storage unavailable")
	ownerPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Repository is read-only. Production DB access must be restricted to trusted
// configuration readers; this is not a READY approval or generation lock.
type Repository struct{ DB *sql.DB }

// Check returns only a safe decision. ownerID is resolved by trusted auth, not
// client query parameters. Evidence.OwnerID identifies its responsible person,
// not necessarily the promoter. Source authenticity is an external audit gate.
func (r Repository) Check(ctx context.Context, ownerID string, key Key, now time.Time) (Decision, error) {
	var reader capabilityReader
	if r.DB != nil {
		reader = r.DB
	}
	return check(ctx, reader, ownerID, key, now)
}

// CheckInTransaction shares the caller's read snapshot without committing it.
// The caller owns isolation and lifecycle; no write or approval is performed.
func CheckInTransaction(ctx context.Context, tx *sql.Tx, ownerID string, key Key, now time.Time) (Decision, error) {
	var reader capabilityReader
	if tx != nil {
		reader = tx
	}
	return check(ctx, reader, ownerID, key, now)
}

type capabilityReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func check(ctx context.Context, reader capabilityReader, ownerID string, key Key, now time.Time) (Decision, error) {
	if !ownerPattern.MatchString(ownerID) || ownerID == "00000000-0000-0000-0000-000000000000" || !validKey(key) || now.IsZero() {
		return Decision{}, ErrInvalid
	}
	if reader == nil {
		return Decision{}, ErrUnavailable
	}
	var memberStatus, positionStatus, positionScene string
	var status, owner, media, source, version, call sql.NullString
	var verified, expires sql.NullTime
	err := reader.QueryRowContext(ctx, `SELECT m.status,p.status,p.scene,c.status,
 e.owner_id,e.media_approval_ref,e.source_approval_ref,e.interface_version,e.real_call_evidence_ref,e.verified_at,e.expires_at
 FROM promotion_positions p JOIN promoter_profiles m ON m.user_id=p.owner_user_id
 LEFT JOIN channel_capabilities c ON c.position_id=p.id AND c.platform=$3 AND c.material_type=$4 AND c.kind=$5
 AND c.media_id=$6 AND c.scene=$7 AND c.terminal=$8 AND c.city_code=$9 AND c.business=$10
 LEFT JOIN channel_capability_evidence e ON e.capability_id=c.id AND e.id=c.evidence_id
 WHERE p.owner_user_id=$1 AND p.id=$2`, ownerID, key.PositionID, key.Platform, key.MaterialType, key.Kind, key.MediaID, key.Scene, key.Terminal, key.CityCode, key.Business).
		Scan(&memberStatus, &positionStatus, &positionScene, &status, &owner, &media, &source, &version, &call, &verified, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Decision{Reason: "POSITION_UNAVAILABLE"}, nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return Decision{}, ctx.Err()
		}
		return Decision{}, ErrUnavailable
	}
	if positionStatus != "ENABLED" || positionScene != key.Scene {
		return Decision{Reason: "POSITION_UNAVAILABLE"}, nil
	}
	if memberStatus != "ENABLED" {
		return Decision{Reason: "NOT_ENABLED"}, nil
	}
	if !status.Valid {
		return Decision{Reason: "UNCONFIGURED"}, nil
	}
	record := Record{Key: key, Status: status.String, Evidence: Evidence{
		OwnerID: owner.String, MediaApprovalRef: media.String, SourceApprovalRef: source.String,
		InterfaceVersion: version.String, RealCallEvidenceRef: call.String,
		VerifiedAt: verified.Time, ExpiresAt: expires.Time,
	}}
	return Evaluate(record, key, now), nil
}
