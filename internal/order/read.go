package order

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"
)

type OrderSummary struct {
	ID                string    `json:"id"`
	Channel           string    `json:"channel"`
	MaskedOrderID     string    `json:"maskedOrderId"`
	Status            string    `json:"orderStatus"`
	PositionID        string    `json:"positionId"`
	AttributionMethod string    `json:"attributionMethod"`
	AttributedAt      time.Time `json:"attributedAt"`
	OrderOccurredAt   time.Time `json:"orderOccurredAt"`
	StatusAt          time.Time `json:"statusAt"`
}

type StatusChange struct {
	PreviousStatus string    `json:"previousStatus"`
	Status         string    `json:"status"`
	OccurredAt     time.Time `json:"occurredAt"`
	ProjectedAt    time.Time `json:"projectedAt"`
}

type OrderDetail struct {
	OrderSummary
	History          []StatusChange `json:"history"`
	RefundEventCount int            `json:"refundEventCount"`
}

type OrderPage struct {
	Items      []OrderSummary `json:"items"`
	NextCursor string         `json:"nextCursor"`
}

type OrderFilter struct {
	OwnerUserID, Channel, PositionID, Status, Cursor string
	From, To                                         time.Time
	Limit                                            int
}

type ReadStore struct{ DB *sql.DB }

var publicIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func maskOrderID(id string) string {
	runes := []rune(id)
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}

func (in OrderFilter) Valid() bool {
	if !validText(in.OwnerUserID, 128) || in.Limit < 1 || in.Limit > 100 ||
		(in.Channel != "" && in.Channel != "JD" && in.Channel != "TB" && in.Channel != "MT") ||
		(in.PositionID != "" && !validText(in.PositionID, 128)) ||
		(in.Status != "" && statusRank[in.Status] == 0 && in.Status != "CANCELLED" && in.Status != "INVALID" && in.Status != "REFUNDED") {
		return false
	}
	if !in.From.IsZero() && !in.To.IsZero() && !in.From.Before(in.To) {
		return false
	}
	_, _, ok := parseCursor(in.Cursor)
	return ok
}

func parseCursor(cursor string) (time.Time, string, bool) {
	if cursor == "" {
		return time.Time{}, "", true
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(raw) > 100 {
		return time.Time{}, "", false
	}
	parts := strings.Split(string(raw), "\x00")
	if len(parts) != 2 {
		return time.Time{}, "", false
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil || !publicIDPattern.MatchString(parts[1]) {
		return time.Time{}, "", false
	}
	return at, parts[1], true
}

func (s ReadStore) ListOwned(ctx context.Context, in OrderFilter) (OrderPage, error) {
	if s.DB == nil || !in.Valid() {
		return OrderPage{}, ErrInvalid
	}
	at, afterID, ok := parseCursor(in.Cursor)
	if !ok {
		return OrderPage{}, ErrInvalid
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT o.public_id::text,a.channel,a.external_order_id,o.status,a.position_id,a.method,a.attributed_at,o.order_occurred_at,o.status_at
		FROM order_attributions a JOIN normalized_orders o ON o.channel=a.channel AND o.external_order_id=a.external_order_id
		WHERE a.owner_user_id=$1 AND a.status='ATTRIBUTED'
		AND ($2='' OR a.channel=$2) AND ($3='' OR a.position_id=$3) AND ($4='' OR o.status=$4)
		AND ($5::timestamptz IS NULL OR o.order_occurred_at >= $5)
		AND ($6::timestamptz IS NULL OR o.order_occurred_at < $6)
		AND ($7::timestamptz IS NULL OR (o.order_occurred_at,o.public_id) < ($7,$8::uuid))
		ORDER BY o.order_occurred_at DESC,o.public_id DESC LIMIT $9`,
		in.OwnerUserID, in.Channel, in.PositionID, in.Status, nullableTime(in.From), nullableTime(in.To), nullableTime(at), nullableUUID(afterID), in.Limit+1)
	if err != nil {
		return OrderPage{}, err
	}
	defer rows.Close()
	page := OrderPage{Items: []OrderSummary{}}
	var lastAt time.Time
	var lastID string
	for rows.Next() {
		var item OrderSummary
		var externalID string
		if err := rows.Scan(&item.ID, &item.Channel, &externalID, &item.Status, &item.PositionID, &item.AttributionMethod, &item.AttributedAt, &item.OrderOccurredAt, &item.StatusAt); err != nil {
			return OrderPage{}, err
		}
		if len(page.Items) == in.Limit {
			page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(lastAt.UTC().Format(time.RFC3339Nano) + "\x00" + lastID))
			break
		}
		item.MaskedOrderID = maskOrderID(externalID)
		page.Items = append(page.Items, item)
		lastAt, lastID = item.OrderOccurredAt, item.ID
	}
	return page, rows.Err()
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s ReadStore) GetOwned(ctx context.Context, owner, id string) (OrderDetail, error) {
	if s.DB == nil || !validText(owner, 128) || !publicIDPattern.MatchString(id) {
		return OrderDetail{}, ErrInvalid
	}
	var detail OrderDetail
	detail.ID = id
	var externalID string
	err := s.DB.QueryRowContext(ctx, `SELECT a.channel,a.external_order_id,o.status,a.position_id,a.method,a.attributed_at,o.order_occurred_at,o.status_at
		FROM order_attributions a JOIN normalized_orders o ON o.channel=a.channel AND o.external_order_id=a.external_order_id
		WHERE a.owner_user_id=$1 AND a.status='ATTRIBUTED' AND o.public_id=$2::uuid`, owner, id).
		Scan(&detail.Channel, &externalID, &detail.Status, &detail.PositionID, &detail.AttributionMethod, &detail.AttributedAt, &detail.OrderOccurredAt, &detail.StatusAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderDetail{}, ErrNotFound
	}
	if err != nil {
		return OrderDetail{}, err
	}
	detail.MaskedOrderID = maskOrderID(externalID)
	rows, err := s.DB.QueryContext(ctx, `SELECT p.previous_status,p.resulting_status,r.occurred_at,p.projected_at
		FROM order_projection_events p JOIN order_raw_events r ON r.id=p.evidence_id
		WHERE p.channel=$1 AND p.external_order_id=$2 AND p.disposition='APPLIED'
		ORDER BY r.occurred_at,p.projected_at`, detail.Channel, externalID)
	if err != nil {
		return OrderDetail{}, err
	}
	defer rows.Close()
	detail.History = []StatusChange{}
	for rows.Next() {
		var change StatusChange
		if err := rows.Scan(&change.PreviousStatus, &change.Status, &change.OccurredAt, &change.ProjectedAt); err != nil {
			return OrderDetail{}, err
		}
		detail.History = append(detail.History, change)
	}
	if err := rows.Err(); err != nil {
		return OrderDetail{}, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM order_refund_events WHERE channel=$1 AND external_order_id=$2`, detail.Channel, externalID).Scan(&detail.RefundEventCount); err != nil {
		return OrderDetail{}, err
	}
	return detail, nil
}
