package conversion

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalid             = errors.New("invalid conversion request")
	ErrNotEnabled          = errors.New("promoter is not enabled")
	ErrPosition            = errors.New("position not found or disabled")
	ErrPreview             = errors.New("preview not found")
	ErrExpired             = errors.New("preview expired")
	ErrIdempotencyConflict = errors.New("conversion idempotency conflict")
	ErrNotFound            = errors.New("conversion not found")
	fingerprintPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type ReserveInput struct {
	OwnerUserID        string
	PositionID         string
	PreviewID          string
	IdempotencyKey     string
	RequestFingerprint string
	Scene              string
	ChannelAccountID   string
	ChannelPositionID  string
}

type Record struct {
	ID                 string
	OwnerUserID        string
	PositionID         string
	PreviewID          string
	TrackingID         string
	IdempotencyKey     string
	RequestFingerprint string
	Scene              string
	Status             string
	ChannelRequestID   string
	Version            int64
	AttemptCount       int
	LeaseExpiresAt     *time.Time
	FailureCode        string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LinkURL            string
	SchemeURL          string
}

type Repository struct{ DB *sql.DB }

func validText(s string, max int) bool {
	return s != "" && len(s) <= max && s == strings.TrimSpace(s)
}

func (in ReserveInput) valid() bool {
	return validText(in.OwnerUserID, 128) && validText(in.PositionID, 128) &&
		validText(in.PreviewID, 128) && validText(in.IdempotencyKey, 128) &&
		validText(in.Scene, 80) && fingerprintPattern.MatchString(in.RequestFingerprint) &&
		validText(in.ChannelAccountID, 128) && validText(in.ChannelPositionID, 128)
}

const insertColumns = `id,owner_user_id,position_id,preview_id,tracking_id,idempotency_key,request_fingerprint,scene,status,created_at,updated_at,link_url,scheme_url`
const recordColumns = insertColumns + `,channel_request_id,version,attempt_count,lease_expires_at,failure_code`

type scanner interface{ Scan(...any) error }

func scanRecord(row scanner) (Record, error) {
	var r Record
	var linkURL, schemeURL, channelRequestID, failureCode sql.NullString
	var leaseExpiresAt sql.NullTime
	err := row.Scan(&r.ID, &r.OwnerUserID, &r.PositionID, &r.PreviewID, &r.TrackingID,
		&r.IdempotencyKey, &r.RequestFingerprint, &r.Scene, &r.Status, &r.CreatedAt,
		&r.UpdatedAt, &linkURL, &schemeURL, &channelRequestID, &r.Version,
		&r.AttemptCount, &leaseExpiresAt, &failureCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	r.LinkURL, r.SchemeURL = linkURL.String, schemeURL.String
	r.ChannelRequestID, r.FailureCode = channelRequestID.String, failureCode.String
	if leaseExpiresAt.Valid {
		r.LeaseExpiresAt = &leaseExpiresAt.Time
	}
	return r, err
}

// Reserve atomically creates a promotion conversion and its legacy-compatible
// Tracking row. It never calls a channel or claims a conversion succeeded.
func (r Repository) Reserve(ctx context.Context, in ReserveInput) (Record, error) {
	if !in.valid() {
		return Record{}, ErrInvalid
	}
	if r.DB == nil {
		return Record{}, errors.New("conversion database unavailable")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	// This is the same owner-row lock used by membership and position writes.
	var membership string
	err = tx.QueryRowContext(ctx, `SELECT status FROM promoter_profiles WHERE user_id=$1 FOR UPDATE`, in.OwnerUserID).Scan(&membership)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotEnabled
	}
	if err != nil {
		return Record{}, err
	}
	if membership != "ENABLED" {
		return Record{}, ErrNotEnabled
	}
	prior, err := scanRecord(tx.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM promotion_conversion_requests WHERE owner_user_id=$1 AND idempotency_key=$2`, in.OwnerUserID, in.IdempotencyKey))
	if err == nil {
		if prior.RequestFingerprint != in.RequestFingerprint {
			return Record{}, ErrIdempotencyConflict
		}
		return prior, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}
	var positionStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM promotion_positions WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, in.PositionID, in.OwnerUserID).Scan(&positionStatus)
	if errors.Is(err, sql.ErrNoRows) || err == nil && positionStatus != "ENABLED" {
		return Record{}, ErrPosition
	}
	if err != nil {
		return Record{}, err
	}
	var productID, previewScene string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT external_product_id,scene,expires_at FROM promotion_previews WHERE id=$1 AND owner_user_id=$2 AND position_id=$3`, in.PreviewID, in.OwnerUserID, in.PositionID).Scan(&productID, &previewScene, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrPreview
	}
	if err != nil {
		return Record{}, err
	}
	if in.Scene != previewScene {
		return Record{}, ErrPreview
	}
	if !expiresAt.After(time.Now()) {
		return Record{}, ErrExpired
	}
	// Lock order matches channel configuration: owner, internal position, mapping.
	// Keep the mapping locked until Tracking and the request commit together.
	var channelStatus, accountID, externalPositionID string
	err = tx.QueryRowContext(ctx, `SELECT status,account_id,external_position_id FROM channel_positions WHERE position_id=$1 AND channel='JD' FOR SHARE`, in.PositionID).Scan(&channelStatus, &accountID, &externalPositionID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (channelStatus != "READY" || accountID != in.ChannelAccountID || externalPositionID != in.ChannelPositionID) {
		return Record{}, ErrNotReady
	}
	if err != nil {
		return Record{}, err
	}
	var conversionID, trackingID [16]byte
	if _, err = rand.Read(conversionID[:]); err != nil {
		return Record{}, err
	}
	if _, err = rand.Read(trackingID[:]); err != nil {
		return Record{}, err
	}
	record := Record{
		ID:          "CR-" + hex.EncodeToString(conversionID[:]),
		TrackingID:  "TR-" + hex.EncodeToString(trackingID[:]),
		OwnerUserID: in.OwnerUserID, PositionID: in.PositionID, PreviewID: in.PreviewID,
		IdempotencyKey: in.IdempotencyKey, RequestFingerprint: in.RequestFingerprint,
		Scene: in.Scene, Status: "PENDING", CreatedAt: time.Now().UTC(),
	}
	record.UpdatedAt = record.CreatedAt
	record.Version = 1
	_, err = tx.ExecContext(ctx, `INSERT INTO tracking_records(id,idempotency_key,user_id,channel,external_product_id,source,created_at)
		VALUES($1,$2,$3,'JD',$4,$5,$6)`, record.TrackingID, "conversion:"+record.ID,
		in.OwnerUserID, productID, "PROMOTION_CENTER", record.CreatedAt)
	if err != nil {
		return Record{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO promotion_conversion_requests(`+insertColumns+`)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, record.ID, record.OwnerUserID,
		record.PositionID, record.PreviewID, record.TrackingID, record.IdempotencyKey,
		record.RequestFingerprint, record.Scene, record.Status, record.CreatedAt, record.UpdatedAt,
		nil, nil)
	if err != nil {
		return Record{}, err
	}
	if err = tx.Commit(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (r Repository) FindByID(ctx context.Context, ownerUserID, id string) (Record, error) {
	return scanRecord(r.DB.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM promotion_conversion_requests WHERE owner_user_id=$1 AND id=$2`, ownerUserID, id))
}

func (r Repository) FindByKey(ctx context.Context, ownerUserID, key string) (Record, error) {
	return scanRecord(r.DB.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM promotion_conversion_requests WHERE owner_user_id=$1 AND idempotency_key=$2`, ownerUserID, key))
}

// FindByIDInternal is for the private channel worker, never an HTTP user lookup.
func (r Repository) FindByIDInternal(ctx context.Context, id string) (Record, error) {
	return scanRecord(r.DB.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM promotion_conversion_requests WHERE id=$1`, id))
}
