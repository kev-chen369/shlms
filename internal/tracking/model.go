package tracking

import (
	"errors"
	"time"
)

type Channel string

const ChannelJD Channel = "JD"

var (
	ErrInvalidInput = errors.New("invalid tracking input")
	ErrNotFound     = errors.New("tracking record not found")
)

type Record struct {
	ID                string
	IdempotencyKey    string
	UserID            string
	Channel           Channel
	ExternalProductID string
	Source            string
	CreatedAt         time.Time
}

type CreateInput struct {
	IdempotencyKey    string
	UserID            string
	Channel           Channel
	ExternalProductID string
	Source            string
}
