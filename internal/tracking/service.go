package tracking

import (
	"context"
	"errors"
	"time"
)

type Repository interface {
	FindByIdempotencyKey(context.Context, string) (Record, error)
	Save(context.Context, Record) error
}

type Service struct {
	repository Repository
	newID      func() string
	now        func() time.Time
}

func NewService(repository Repository, newID func() string, now func() time.Time) Service {
	return Service{repository: repository, newID: newID, now: now}
}

func (s Service) Create(ctx context.Context, input CreateInput) (Record, error) {
	if input.IdempotencyKey == "" ||
		input.UserID == "" ||
		input.Channel != ChannelJD ||
		input.ExternalProductID == "" {
		return Record{}, ErrInvalidInput
	}

	record, err := s.repository.FindByIdempotencyKey(ctx, input.IdempotencyKey)
	if err == nil {
		if !sameRequest(record, input) {
			return Record{}, ErrConflict
		}
		return record, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Record{}, err
	}

	record = Record{
		ID:                s.newID(),
		IdempotencyKey:    input.IdempotencyKey,
		UserID:            input.UserID,
		Channel:           input.Channel,
		ExternalProductID: input.ExternalProductID,
		Source:            input.Source,
		CreatedAt:         s.now(),
	}
	if err := s.repository.Save(ctx, record); err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			existing, findErr := s.repository.FindByIdempotencyKey(ctx, input.IdempotencyKey)
			if findErr != nil {
				return Record{}, findErr
			}
			if !sameRequest(existing, input) {
				return Record{}, ErrConflict
			}
			return existing, nil
		}
		return Record{}, err
	}
	return record, nil
}

func sameRequest(record Record, input CreateInput) bool {
	return record.UserID == input.UserID &&
		record.Channel == input.Channel &&
		record.ExternalProductID == input.ExternalProductID &&
		record.Source == input.Source
}
