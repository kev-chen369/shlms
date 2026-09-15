package tracking

import (
	"context"
	"database/sql"
	"errors"
)

// PostgresRepository stores Tracking records after the tracking_records migration.
type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) PostgresRepository {
	return PostgresRepository{db: db}
}

func (r PostgresRepository) FindByIdempotencyKey(ctx context.Context, key string) (Record, error) {
	var record Record
	err := r.db.QueryRowContext(ctx, `
		SELECT id, idempotency_key, user_id, channel, external_product_id, source, created_at
		FROM tracking_records WHERE idempotency_key = $1`, key).Scan(
		&record.ID, &record.IdempotencyKey, &record.UserID, &record.Channel,
		&record.ExternalProductID, &record.Source, &record.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

func (r PostgresRepository) Save(ctx context.Context, record Record) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO tracking_records
			(id, idempotency_key, user_id, channel, external_product_id, source, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		record.ID, record.IdempotencyKey, record.UserID, record.Channel,
		record.ExternalProductID, record.Source, record.CreatedAt,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAlreadyExists
	}
	return nil
}
