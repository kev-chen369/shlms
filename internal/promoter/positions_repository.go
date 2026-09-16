package promoter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type rowScanner interface{ Scan(...any) error }

func scanPosition(row rowScanner) (Position, error) {
	var p Position
	err := row.Scan(&p.ID, &p.OwnerUserID, &p.Name, &p.Scene, &p.Status, &p.IsDefault, &p.Version, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Position{}, ErrNotFound
	}
	return p, err
}

const positionColumns = `id,owner_user_id,name,scene,status,is_default,version,created_at`

func (r PostgresRepository) ChangePosition(ctx context.Context, userID string, c PositionCommand) (Position, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return Position{}, err
	}
	sum := sha256.Sum256(b)
	fingerprint := hex.EncodeToString(sum[:])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Position{}, err
	}
	defer tx.Rollback()
	// All position writes and membership reviews lock the same owner row first.
	var membership Status
	err = tx.QueryRowContext(ctx, `SELECT status FROM promoter_profiles WHERE user_id=$1 FOR UPDATE`, userID).Scan(&membership)
	if errors.Is(err, sql.ErrNoRows) {
		return Position{}, ErrNotEnabled
	}
	if err != nil {
		return Position{}, err
	}
	if membership != Enabled {
		return Position{}, ErrNotEnabled
	}
	var storedFingerprint string
	var result []byte
	err = tx.QueryRowContext(ctx, `SELECT request_fingerprint,result FROM promotion_position_requests WHERE owner_user_id=$1 AND action=$2 AND idempotency_key=$3`, userID, c.Action, c.IdempotencyKey).Scan(&storedFingerprint, &result)
	if err == nil {
		if storedFingerprint != fingerprint {
			return Position{}, ErrIdempotencyConflict
		}
		var p Position
		if err = json.Unmarshal(result, &p); err != nil {
			return Position{}, err
		}
		p.OwnerUserID = userID
		if err = tx.Commit(); err != nil {
			return Position{}, err
		}
		return p, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Position{}, err
	}
	var p Position
	if c.Action == PositionCreate {
		var id [16]byte
		if _, err = rand.Read(id[:]); err != nil {
			return Position{}, err
		}
		p = Position{ID: "PP-" + hex.EncodeToString(id[:]), OwnerUserID: userID, Name: c.Name, Scene: c.Scene, Status: Enabled, IsDefault: c.IsDefault, Version: 1}
	} else {
		p, err = scanPosition(tx.QueryRowContext(ctx, `SELECT `+positionColumns+` FROM promotion_positions WHERE id=$1 AND owner_user_id=$2 FOR UPDATE`, c.ID, userID))
		if err != nil {
			return Position{}, err
		}
		if p.Version != c.Version {
			return Position{}, ErrConflict
		}
		if p.Status != Enabled {
			return Position{}, ErrTransition
		}
		switch c.Action {
		case PositionEdit:
			p.Name, p.Scene = c.Name, c.Scene
		case PositionDefault:
			p.IsDefault = true
		case PositionDisable:
			p.Status, p.IsDefault = Disabled, false
		default:
			return Position{}, ErrInvalidInput
		}
		p.Version++
	}
	if p.IsDefault {
		// Invalidates old editors of the previous default as well as its flag.
		if _, err = tx.ExecContext(ctx, `UPDATE promotion_positions SET is_default=false,version=version+1 WHERE owner_user_id=$1 AND is_default AND id<>$2`, userID, p.ID); err != nil {
			return Position{}, err
		}
	}
	if c.Action == PositionCreate {
		p, err = scanPosition(tx.QueryRowContext(ctx, `INSERT INTO promotion_positions(id,owner_user_id,name,scene,status,is_default,version) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+positionColumns, p.ID, userID, p.Name, p.Scene, p.Status, p.IsDefault, p.Version))
	} else {
		p, err = scanPosition(tx.QueryRowContext(ctx, `UPDATE promotion_positions SET name=$3,scene=$4,status=$5,is_default=$6,version=$7 WHERE id=$1 AND owner_user_id=$2 RETURNING `+positionColumns, p.ID, userID, p.Name, p.Scene, p.Status, p.IsDefault, p.Version))
	}
	if err != nil {
		return Position{}, err
	}
	p.ChannelReadiness, err = positionChannelReadiness(ctx, tx, p)
	if err != nil {
		return Position{}, err
	}
	result, err = json.Marshal(p)
	if err != nil {
		return Position{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO promotion_position_requests(owner_user_id,action,idempotency_key,request_fingerprint,result) VALUES($1,$2,$3,$4,$5)`, userID, c.Action, c.IdempotencyKey, fingerprint, string(result))
	if err != nil {
		return Position{}, err
	}
	if err = tx.Commit(); err != nil {
		return Position{}, err
	}
	return p, nil
}

func positionChannelReadiness(ctx context.Context, tx *sql.Tx, p Position) (string, error) {
	if p.Status == Disabled {
		return "UNAVAILABLE", nil
	}
	var mappingStatus string
	err := tx.QueryRowContext(ctx, `SELECT status FROM channel_positions WHERE position_id=$1 AND channel='JD'`, p.ID).Scan(&mappingStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return "WAITING_CONFIGURATION", nil
	}
	if err != nil {
		return "", err
	}
	if mappingStatus == "PENDING_VERIFICATION" {
		return "WAITING_VERIFICATION", nil
	}
	return "UNAVAILABLE", nil
}

type positionCursor struct {
	UserID string
	Status Status
	ID     string
}

func (r PostgresRepository) ListPositions(ctx context.Context, userID string, in PositionListInput) (PositionPage, error) {
	if in.Limit < 1 || in.Limit > 100 {
		return PositionPage{}, ErrInvalidInput
	}
	afterID := ""
	if in.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		if err != nil {
			return PositionPage{}, ErrInvalidInput
		}
		var c positionCursor
		if err = json.Unmarshal(b, &c); err != nil || c.UserID != userID || c.Status != in.Status || !validText(c.ID, 256) {
			return PositionPage{}, ErrInvalidInput
		}
		afterID = c.ID
	}
	// Historical reads remain available even after membership is disabled.
	rows, err := r.db.QueryContext(ctx, `SELECT p.id,p.owner_user_id,p.name,p.scene,p.status,p.is_default,p.version,p.created_at,
		CASE WHEN p.status='DISABLED' OR m.status<>'ENABLED' THEN 'UNAVAILABLE' WHEN c.status='PENDING_VERIFICATION' THEN 'WAITING_VERIFICATION'
		ELSE 'WAITING_CONFIGURATION' END AS jd_readiness
		FROM promotion_positions p JOIN promoter_profiles m ON m.user_id=p.owner_user_id
		LEFT JOIN channel_positions c ON c.position_id=p.id AND c.channel='JD'
		WHERE p.owner_user_id=$1 AND ($2='' OR p.status=$2) AND p.id>$3 ORDER BY p.id LIMIT $4`, userID, in.Status, afterID, in.Limit+1)
	if err != nil {
		return PositionPage{}, err
	}
	defer rows.Close()
	page := PositionPage{Items: []Position{}}
	for rows.Next() {
		var p Position
		err := rows.Scan(&p.ID, &p.OwnerUserID, &p.Name, &p.Scene, &p.Status, &p.IsDefault, &p.Version, &p.CreatedAt, &p.ChannelReadiness)
		if err != nil {
			return PositionPage{}, err
		}
		page.Items = append(page.Items, p)
	}
	if err = rows.Err(); err != nil {
		return PositionPage{}, err
	}
	if len(page.Items) > in.Limit {
		page.Items = page.Items[:in.Limit]
		last := page.Items[len(page.Items)-1]
		b, err := json.Marshal(positionCursor{UserID: userID, Status: in.Status, ID: last.ID})
		if err != nil {
			return PositionPage{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	return page, nil
}
