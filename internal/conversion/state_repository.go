package conversion

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"time"

	"github.com/kev-chen369/shlms/internal/linkresolve"
)

var ErrStateConflict = errors.New("conversion state conflict")

// Claim assigns a stable channel request ID before any external call. Only a
// PENDING request can be claimed; uncertain work is recovered by query, not resend.
func (r Repository) Claim(ctx context.Context, id string, version int64, lease time.Duration) (Record, error) {
	if !validText(id, 128) || version < 1 || lease <= 0 || lease > 5*time.Minute || r.DB == nil {
		return Record{}, ErrInvalid
	}
	return transition(r.DB.QueryRowContext(ctx, `UPDATE promotion_conversion_requests AS cr
		SET status='PROCESSING',channel_request_id=cr.id,attempt_count=attempt_count+1,
			lease_expires_at=$3,version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE cr.id=$1 AND cr.version=$2 AND cr.status='PENDING'
			AND EXISTS (SELECT 1 FROM promoter_profiles m WHERE m.user_id=cr.owner_user_id AND m.status='ENABLED')
			AND EXISTS (SELECT 1 FROM promotion_positions p WHERE p.id=cr.position_id AND p.owner_user_id=cr.owner_user_id AND p.status='ENABLED')
		RETURNING `+recordColumns, id, version, time.Now().Add(lease)))
}

// MarkUncertain records a timeout or unknown outcome. A recovery worker must
// query the channel by ChannelRequestID before any further action.
func (r Repository) MarkUncertain(ctx context.Context, id, channelRequestID string, version int64) (Record, error) {
	if !validText(id, 128) || channelRequestID != id || version < 1 || r.DB == nil {
		return Record{}, ErrInvalid
	}
	return transition(r.DB.QueryRowContext(ctx, `UPDATE promotion_conversion_requests
		SET status='FAILED_RETRYABLE',lease_expires_at=NULL,failure_code='QUERY_REQUIRED',
			version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE id=$1 AND channel_request_id=$2 AND version=$3 AND status='PROCESSING'
		RETURNING `+recordColumns, id, channelRequestID, version))
}

func (r Repository) MarkFinal(ctx context.Context, id, channelRequestID string, version int64, code string) (Record, error) {
	if !validText(id, 128) || channelRequestID != id || version < 1 || r.DB == nil ||
		(code != "CHANNEL_REJECTED" && code != "INVALID_RESULT") {
		return Record{}, ErrInvalid
	}
	return transition(r.DB.QueryRowContext(ctx, `UPDATE promotion_conversion_requests
		SET status='FAILED_FINAL',lease_expires_at=NULL,failure_code=$4,
			version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE id=$1 AND channel_request_id=$2 AND version=$3 AND status IN ('PROCESSING','FAILED_RETRYABLE')
		RETURNING `+recordColumns, id, channelRequestID, version, code))
}

// RejectPending is used when the pre-call quote changed. No channel request is sent.
func (r Repository) RejectPending(ctx context.Context, id string, version int64) (Record, error) {
	if !validText(id, 128) || version < 1 || r.DB == nil {
		return Record{}, ErrInvalid
	}
	return transition(r.DB.QueryRowContext(ctx, `UPDATE promotion_conversion_requests
		SET status='FAILED_FINAL',channel_request_id=id,failure_code='INVALID_RESULT',
			version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE id=$1 AND version=$2 AND status='PENDING'
		RETURNING `+recordColumns, id, version))
}

// MarkSucceeded accepts a late verified channel result even after an uncertain
// timeout. The caller must validate the URL against the approved channel host.
func (r Repository) MarkSucceeded(ctx context.Context, id, channelRequestID string, version int64, linkURL string) (Record, error) {
	if !validText(id, 128) || channelRequestID != id || version < 1 || r.DB == nil || !safeHTTPSLink(linkURL) {
		return Record{}, ErrInvalid
	}
	return transition(r.DB.QueryRowContext(ctx, `UPDATE promotion_conversion_requests
		SET status='SUCCEEDED',lease_expires_at=NULL,failure_code=NULL,link_url=$4,
			version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE id=$1 AND channel_request_id=$2 AND version=$3 AND status IN ('PROCESSING','FAILED_RETRYABLE')
		RETURNING `+recordColumns, id, channelRequestID, version, linkURL))
}

func safeHTTPSLink(raw string) bool {
	if len(raw) == 0 || len(raw) > 4096 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return false
	}
	policy, err := linkresolve.NewPolicy([]string{u.Hostname()})
	if err != nil {
		return false
	}
	_, err = policy.Validate(raw)
	return err == nil
}

func transition(row *sql.Row) (Record, error) {
	record, err := scanRecord(row)
	if errors.Is(err, ErrNotFound) {
		return Record{}, ErrStateConflict
	}
	return record, err
}

// RecoveryCandidates is read-only: it never claims or resends uncertain work.
func (r Repository) RecoveryCandidates(ctx context.Context, limit int) ([]Record, error) {
	if r.DB == nil || limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+recordColumns+` FROM promotion_conversion_requests
		WHERE status='FAILED_RETRYABLE' OR (status='PROCESSING' AND lease_expires_at <= CURRENT_TIMESTAMP)
		ORDER BY updated_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
