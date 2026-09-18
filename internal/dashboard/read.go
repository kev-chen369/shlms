package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrInvalid = errors.New("invalid dashboard filter")

type Filter struct {
	OwnerUserID, Channel, PositionID string
	From, To                         time.Time // Inclusive start and exclusive end in Asia/Shanghai.
}

func (f Filter) Valid() bool {
	return f.OwnerUserID != "" && len(f.OwnerUserID) <= 128 && (f.Channel == "" || f.Channel == "JD" || f.Channel == "TB" || f.Channel == "MT") && len(f.PositionID) <= 128 && !f.From.IsZero() && !f.To.IsZero() && f.From.Before(f.To) && f.To.Sub(f.From) <= 366*24*time.Hour
}

type Counts struct {
	TimeZone        string    `json:"timeZone"`
	From            time.Time `json:"from"`
	ToExclusive     time.Time `json:"toExclusive"`
	AsOf            time.Time `json:"asOf"`
	SuccessfulLinks int64     `json:"successfulLinks"`
	CopyReports     int64     `json:"copyReports"`
	ValidOrders     int64     `json:"validOrders"`
}

type ReadStore struct{ DB *sql.DB }

func (s ReadStore) Get(ctx context.Context, f Filter) (Counts, error) {
	if s.DB == nil || !f.Valid() {
		return Counts{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return Counts{}, err
	}
	defer tx.Rollback()
	out := Counts{TimeZone: "Asia/Shanghai", From: f.From, ToExclusive: f.To}
	if err = tx.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&out.AsOf); err != nil {
		return Counts{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM promotion_conversion_requests c JOIN promotion_previews p ON p.id=c.preview_id
  WHERE c.owner_user_id=$1 AND c.status='SUCCEEDED' AND c.updated_at >= $2 AND c.updated_at < $3
  AND ($4='' OR p.channel=$4) AND ($5='' OR c.position_id=$5)`, f.OwnerUserID, f.From, f.To, f.Channel, f.PositionID).Scan(&out.SuccessfulLinks)
	if err != nil {
		return Counts{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM promotion_share_events e JOIN promotion_conversion_requests c ON c.id=e.conversion_request_id JOIN promotion_previews p ON p.id=c.preview_id
  WHERE e.owner_user_id=$1 AND e.recorded_at >= $2 AND e.recorded_at < $3 AND e.action='COPY_REPORTED'
  AND ($4='' OR p.channel=$4) AND ($5='' OR c.position_id=$5)`, f.OwnerUserID, f.From, f.To, f.Channel, f.PositionID).Scan(&out.CopyReports)
	if err != nil {
		return Counts{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM order_attributions a JOIN normalized_orders o ON o.channel=a.channel AND o.external_order_id=a.external_order_id
  WHERE a.owner_user_id=$1 AND a.status='ATTRIBUTED' AND o.status NOT IN ('INVALID','CANCELLED','REFUNDED')
  AND o.order_occurred_at >= $2 AND o.order_occurred_at < $3
  AND ($4='' OR a.channel=$4) AND ($5='' OR a.position_id=$5)`, f.OwnerUserID, f.From, f.To, f.Channel, f.PositionID).Scan(&out.ValidOrders)
	if err != nil {
		return Counts{}, err
	}
	if err = tx.Commit(); err != nil {
		return Counts{}, err
	}
	return out, nil
}
