package order

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

var (
	ErrProjectionInvalid = errors.New("invalid order projection")
	ErrMissingOrder      = errors.New("refund has no known order")
	ErrTransition        = errors.New("order transition is not allowed")
)

type ProjectionInput struct {
	EvidenceID, Status, RefundID, RefundKind string
	RefundAmountMinor                        int64
}

type ProjectionResult struct {
	Channel, ExternalOrderID, PreviousStatus, Status, Disposition string
}

type ProjectionStore struct{ DB *sql.DB }

var statusRank = map[string]int{
	"CREATED": 1, "PAID": 2, "CONFIRMED": 3, "COMMISSION_CONFIRMED": 4,
	"SETTLEMENT_PENDING": 5, "SETTLED": 6,
}

func transition(current, next string) bool {
	if current == next {
		return false
	}
	if current == "CANCELLED" || current == "INVALID" || current == "REFUNDED" {
		return false
	}
	if next == "INVALID" {
		return true
	}
	if next == "CANCELLED" {
		return current == "CREATED" || current == "PAID"
	}
	if next == "REFUNDED" {
		return false
	} // Only a verified refund event can reach this status.
	return statusRank[next] > statusRank[current] && statusRank[current] > 0
}

// Apply consumes an already persisted channel event. It never attributes a
// buyer or promoter, calculates commission, or infers an order from a click.
func (s ProjectionStore) Apply(ctx context.Context, in ProjectionInput) (ProjectionResult, error) {
	if s.DB == nil || !validText(in.EvidenceID, 128) {
		return ProjectionResult{}, ErrProjectionInvalid
	}
	if in.Status != "" {
		if statusRank[in.Status] == 0 && in.Status != "CANCELLED" && in.Status != "INVALID" || in.RefundID != "" || in.RefundKind != "" || in.RefundAmountMinor != 0 {
			return ProjectionResult{}, ErrProjectionInvalid
		}
	} else if !validText(in.RefundID, 128) || (in.RefundKind != "PARTIAL" && in.RefundKind != "FULL") || in.RefundAmountMinor <= 0 {
		return ProjectionResult{}, ErrProjectionInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProjectionResult{}, err
	}
	defer tx.Rollback()
	var channel, externalID, eventType string
	var occurredAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT channel,external_order_id,event_type,occurred_at FROM order_raw_events WHERE id=$1`, in.EvidenceID).
		Scan(&channel, &externalID, &eventType, &occurredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectionResult{}, ErrNotFound
	}
	if err != nil {
		return ProjectionResult{}, err
	}
	if (in.Status != "" && eventType != "ORDER") || (in.Status == "" && eventType != "REFUND") {
		return ProjectionResult{}, ErrProjectionInvalid
	}
	fingerprintBytes := sha256.Sum256([]byte(in.Status + "\x00" + in.RefundID + "\x00" + in.RefundKind + "\x00" + strconv.FormatInt(in.RefundAmountMinor, 10)))
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	// Per-order transaction lock handles simultaneous status and refund events.
	lockKey := sha256.Sum256([]byte(channel + "\x00" + externalID))
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(9441, hashtext($1))`, hex.EncodeToString(lockKey[:])); err != nil {
		return ProjectionResult{}, err
	}
	var existing ProjectionResult
	var storedFingerprint string
	err = tx.QueryRowContext(ctx, `SELECT channel,external_order_id,previous_status,resulting_status,disposition,input_fingerprint
		FROM order_projection_events WHERE evidence_id=$1`, in.EvidenceID).
		Scan(&existing.Channel, &existing.ExternalOrderID, &existing.PreviousStatus, &existing.Status, &existing.Disposition, &storedFingerprint)
	if err == nil {
		if storedFingerprint != fingerprint {
			return ProjectionResult{}, ErrConflict
		}
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ProjectionResult{}, err
	}
	var current string
	var statusAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT status,status_at FROM normalized_orders WHERE channel=$1 AND external_order_id=$2 FOR UPDATE`, channel, externalID).
		Scan(&current, &statusAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ProjectionResult{}, err
	}
	orderExists := err == nil
	result := ProjectionResult{Channel: channel, ExternalOrderID: externalID, PreviousStatus: current, Status: current}
	if in.Status != "" {
		if !orderExists {
			_, err = tx.ExecContext(ctx, `INSERT INTO normalized_orders(channel,external_order_id,status,status_at,order_occurred_at,latest_evidence_id)
				VALUES($1,$2,$3,$4,$4,$5)`, channel, externalID, in.Status, occurredAt, in.EvidenceID)
			if err != nil {
				return ProjectionResult{}, err
			}
			result.Status, result.Disposition = in.Status, "APPLIED"
		} else {
			if _, err = tx.ExecContext(ctx, `UPDATE normalized_orders SET order_occurred_at=LEAST(order_occurred_at,$3)
				WHERE channel=$1 AND external_order_id=$2`, channel, externalID, occurredAt); err != nil {
				return ProjectionResult{}, err
			}
			switch {
			case in.Status == current:
				result.Disposition = "DUPLICATE"
				if occurredAt.After(statusAt) {
					_, err = tx.ExecContext(ctx, `UPDATE normalized_orders SET status_at=$3,latest_evidence_id=$4,updated_at=CURRENT_TIMESTAMP
						WHERE channel=$1 AND external_order_id=$2`, channel, externalID, occurredAt, in.EvidenceID)
					if err != nil {
						return ProjectionResult{}, err
					}
				}
			case occurredAt.Before(statusAt):
				result.Disposition = "STALE"
			case transition(current, in.Status):
				_, err = tx.ExecContext(ctx, `UPDATE normalized_orders SET status=$3,status_at=$4,latest_evidence_id=$5,updated_at=CURRENT_TIMESTAMP
					WHERE channel=$1 AND external_order_id=$2`, channel, externalID, in.Status, occurredAt, in.EvidenceID)
				if err != nil {
					return ProjectionResult{}, err
				}
				result.Status, result.Disposition = in.Status, "APPLIED"
			default:
				result.Disposition = "INVALID_TRANSITION"
			}
		}
	} else {
		if !orderExists {
			return ProjectionResult{}, ErrMissingOrder
		}
		if current == "CANCELLED" || current == "INVALID" || current == "REFUNDED" || current == "CREATED" {
			return ProjectionResult{}, ErrTransition
		}
		var priorEvidence string
		err = tx.QueryRowContext(ctx, `SELECT evidence_id FROM order_refund_events WHERE channel=$1 AND refund_id=$2`, channel, in.RefundID).Scan(&priorEvidence)
		if err == nil {
			return ProjectionResult{}, ErrConflict
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return ProjectionResult{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO order_refund_events(evidence_id,channel,external_order_id,refund_id,amount_minor,kind,occurred_at)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, in.EvidenceID, channel, externalID, in.RefundID, in.RefundAmountMinor, in.RefundKind, occurredAt)
		if err != nil {
			return ProjectionResult{}, err
		}
		result.Disposition = "APPLIED"
		if in.RefundKind == "FULL" {
			newAt := occurredAt
			if statusAt.After(newAt) {
				newAt = statusAt
			}
			_, err = tx.ExecContext(ctx, `UPDATE normalized_orders SET status='REFUNDED',status_at=$3,latest_evidence_id=$4,updated_at=CURRENT_TIMESTAMP
				WHERE channel=$1 AND external_order_id=$2`, channel, externalID, newAt, in.EvidenceID)
			if err != nil {
				return ProjectionResult{}, err
			}
			result.Status = "REFUNDED"
		}
	}
	mapped := in.Status
	if mapped == "" {
		mapped = in.RefundKind + "_REFUND"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO order_projection_events
		(evidence_id,channel,external_order_id,mapped_status,input_fingerprint,previous_status,resulting_status,disposition)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.EvidenceID, channel, externalID, mapped, fingerprint, result.PreviousStatus, result.Status, result.Disposition)
	if err != nil {
		return ProjectionResult{}, err
	}
	return result, tx.Commit()
}
