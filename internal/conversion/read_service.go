package conversion

import (
	"context"
)

type RecordReader interface {
	FindByID(context.Context, string, string) (Record, error)
}

// ReadService exposes historic status to its owner even after membership is disabled.
type ReadService struct{ Repository RecordReader }

func (s ReadService) Get(ctx context.Context, ownerUserID, id string) (Record, error) {
	if !validText(ownerUserID, 128) || !validText(id, 128) {
		return Record{}, ErrInvalid
	}
	if s.Repository == nil {
		return Record{}, ErrUnavailable
	}
	r, err := s.Repository.FindByID(ctx, ownerUserID, id)
	if err != nil {
		return Record{}, err
	}
	if r.OwnerUserID != ownerUserID || r.ID != id {
		return Record{}, ErrNotFound
	}
	return r, nil
}
