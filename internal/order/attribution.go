package order

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrAttributionInvalid  = errors.New("invalid order attribution")
	ErrAttributionConflict = errors.New("order attribution conflicts with existing evidence")
)

const (
	AttributionSubID           = "SUB_ID"
	AttributionLinkRequest     = "LINK_REQUEST"
	AttributionChannelPosition = "CHANNEL_POSITION"
	AttributionNone            = "NONE"
)

type AttributionInput struct {
	EvidenceID, Method, Value, AccountID string
}

type Attribution struct {
	Channel, ExternalOrderID, Status, Method                 string
	OwnerUserID, PositionID, TrackingID, ConversionRequestID string
}

type AttributionStore struct{ DB *sql.DB }

func (in AttributionInput) valid() bool {
	if !validText(in.EvidenceID, 128) {
		return false
	}
	switch in.Method {
	case AttributionNone:
		return in.Value == "" && in.AccountID == ""
	case AttributionSubID, AttributionLinkRequest:
		return validText(in.Value, 128) && in.AccountID == ""
	case AttributionChannelPosition:
		return validText(in.Value, 128) && validText(in.AccountID, 128)
	default:
		return false
	}
}

func attributionFingerprint(in AttributionInput) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{in.Method, in.Value, in.AccountID}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// Apply records only exact, database-verifiable attribution. Unknown or absent
// evidence is retained for review and never falls back to time or amount.
func (s AttributionStore) Apply(ctx context.Context, in AttributionInput) (Attribution, error) {
	if s.DB == nil || !in.valid() {
		return Attribution{}, ErrAttributionInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Attribution{}, err
	}
	defer tx.Rollback()

	var channel, externalID string
	err = tx.QueryRowContext(ctx, `SELECT r.channel,r.external_order_id
		FROM order_raw_events r
		JOIN normalized_orders o ON o.channel=r.channel AND o.external_order_id=r.external_order_id
		WHERE r.id=$1`, in.EvidenceID).Scan(&channel, &externalID)
	if errors.Is(err, sql.ErrNoRows) {
		return Attribution{}, ErrNotFound
	}
	if err != nil {
		return Attribution{}, err
	}
	lockKey := sha256.Sum256([]byte(channel + "\x00" + externalID))
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(9442, hashtext($1))`, hex.EncodeToString(lockKey[:])); err != nil {
		return Attribution{}, err
	}
	fingerprint := attributionFingerprint(in)
	var existing Attribution
	var storedFingerprint string
	err = tx.QueryRowContext(ctx, `SELECT channel,external_order_id,status,method,
		COALESCE(owner_user_id,''),COALESCE(position_id,''),COALESCE(tracking_id,''),COALESCE(conversion_request_id,''),evidence_value_hash
		FROM order_attributions WHERE channel=$1 AND external_order_id=$2`, channel, externalID).
		Scan(&existing.Channel, &existing.ExternalOrderID, &existing.Status, &existing.Method,
			&existing.OwnerUserID, &existing.PositionID, &existing.TrackingID, &existing.ConversionRequestID, &storedFingerprint)
	if err == nil {
		if storedFingerprint != fingerprint || existing.Method != in.Method {
			return Attribution{}, ErrAttributionConflict
		}
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Attribution{}, err
	}

	result := Attribution{Channel: channel, ExternalOrderID: externalID, Status: "PENDING_REVIEW", Method: in.Method}
	switch in.Method {
	case AttributionSubID:
		err = tx.QueryRowContext(ctx, `SELECT c.owner_user_id,c.position_id,c.tracking_id,c.id
			FROM promotion_conversion_requests c JOIN tracking_records t ON t.id=c.tracking_id
			WHERE c.tracking_id=$1 AND t.channel=$2`, in.Value, channel).
			Scan(&result.OwnerUserID, &result.PositionID, &result.TrackingID, &result.ConversionRequestID)
	case AttributionLinkRequest:
		err = tx.QueryRowContext(ctx, `SELECT c.owner_user_id,c.position_id,c.tracking_id,c.id
			FROM promotion_conversion_requests c JOIN tracking_records t ON t.id=c.tracking_id
			WHERE c.channel_request_id=$1 AND c.status='SUCCEEDED' AND t.channel=$2`, in.Value, channel).
			Scan(&result.OwnerUserID, &result.PositionID, &result.TrackingID, &result.ConversionRequestID)
	case AttributionChannelPosition:
		err = tx.QueryRowContext(ctx, `SELECT p.owner_user_id,p.id
			FROM channel_positions cp JOIN promotion_positions p ON p.id=cp.position_id
			WHERE cp.channel=$1 AND cp.account_id=$2 AND cp.external_position_id=$3 AND cp.status='READY'`,
			channel, in.AccountID, in.Value).Scan(&result.OwnerUserID, &result.PositionID)
	case AttributionNone:
		err = sql.ErrNoRows
	}
	if err == nil {
		result.Status = "ATTRIBUTED"
	} else if errors.Is(err, sql.ErrNoRows) {
		result.OwnerUserID, result.PositionID, result.TrackingID, result.ConversionRequestID = "", "", "", ""
	} else {
		return Attribution{}, err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO order_attributions
		(channel,external_order_id,evidence_id,status,method,evidence_value_hash,owner_user_id,position_id,tracking_id,conversion_request_id)
		VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''))`,
		channel, externalID, in.EvidenceID, result.Status, result.Method, fingerprint,
		result.OwnerUserID, result.PositionID, result.TrackingID, result.ConversionRequestID)
	if err != nil {
		return Attribution{}, err
	}
	return result, tx.Commit()
}
