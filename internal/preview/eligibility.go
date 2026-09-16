package preview

import (
	"context"
	"database/sql"
	"errors"
)

type PostgresEligibility struct{ DB *sql.DB }

func (p PostgresEligibility) Check(ctx context.Context, ownerUserID, positionID string) (ChannelPosition, error) {
	if p.DB == nil {
		return ChannelPosition{}, ErrUnavailable
	}
	var membership, positionStatus, channelStatus string
	var accountID, externalPositionID sql.NullString
	err := p.DB.QueryRowContext(ctx, `SELECT m.status,p.status,COALESCE(c.status,''),c.account_id,c.external_position_id
		FROM promotion_positions p JOIN promoter_profiles m ON m.user_id=p.owner_user_id
		LEFT JOIN channel_positions c ON c.position_id=p.id AND c.channel='JD'
		WHERE p.id=$1 AND p.owner_user_id=$2`, positionID, ownerUserID).
		Scan(&membership, &positionStatus, &channelStatus, &accountID, &externalPositionID)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelPosition{}, ErrPosition
	}
	if err != nil {
		return ChannelPosition{}, err
	}
	if membership != "ENABLED" {
		return ChannelPosition{}, ErrNotEnabled
	}
	if positionStatus != "ENABLED" {
		return ChannelPosition{}, ErrPosition
	}
	if channelStatus != "READY" || !accountID.Valid || !externalPositionID.Valid {
		return ChannelPosition{}, ErrNotReady
	}
	return ChannelPosition{AccountID: accountID.String, ExternalPositionID: externalPositionID.String}, nil
}
