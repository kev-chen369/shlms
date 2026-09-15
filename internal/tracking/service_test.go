package tracking

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepository struct {
	records   map[string]Record
	saveCalls int
}

func (r *memoryRepository) FindByIdempotencyKey(_ context.Context, key string) (Record, error) {
	record, ok := r.records[key]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

func (r *memoryRepository) Save(_ context.Context, record Record) error {
	r.saveCalls++
	r.records[record.IdempotencyKey] = record
	return nil
}

func TestCreateReturnsExistingRecordForSameIdempotencyKey(t *testing.T) {
	existing := Record{
		ID:                "TRK-existing",
		IdempotencyKey:    "request-1",
		UserID:            "user-1",
		Channel:           ChannelJD,
		ExternalProductID: "sku-1",
		Source:            "product_detail",
		CreatedAt:         time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
	}
	repository := &memoryRepository{records: map[string]Record{"request-1": existing}}
	service := NewService(repository, func() string { return "TRK-new" }, time.Now)

	got, err := service.Create(context.Background(), CreateInput{
		IdempotencyKey:    "request-1",
		UserID:            "user-1",
		Channel:           ChannelJD,
		ExternalProductID: "sku-1",
		Source:            "product_detail",
	})

	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got != existing {
		t.Fatalf("record = %#v, want %#v", got, existing)
	}
	if repository.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0", repository.saveCalls)
	}
}

func TestCreateSavesNewRecord(t *testing.T) {
	createdAt := time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC)
	repository := &memoryRepository{records: make(map[string]Record)}
	service := NewService(repository, func() string { return "TRK-new" }, func() time.Time { return createdAt })

	got, err := service.Create(context.Background(), CreateInput{
		IdempotencyKey:    "request-2",
		UserID:            "user-2",
		Channel:           ChannelJD,
		ExternalProductID: "sku-2",
		Source:            "search",
	})

	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	want := Record{
		ID:                "TRK-new",
		IdempotencyKey:    "request-2",
		UserID:            "user-2",
		Channel:           ChannelJD,
		ExternalProductID: "sku-2",
		Source:            "search",
		CreatedAt:         createdAt,
	}
	if got != want {
		t.Fatalf("record = %#v, want %#v", got, want)
	}
	if repository.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", repository.saveCalls)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	valid := CreateInput{
		IdempotencyKey:    "request-1",
		UserID:            "user-1",
		Channel:           ChannelJD,
		ExternalProductID: "sku-1",
		Source:            "product_detail",
	}
	tests := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{name: "blank idempotency key", mutate: func(in *CreateInput) { in.IdempotencyKey = "" }},
		{name: "blank user", mutate: func(in *CreateInput) { in.UserID = "" }},
		{name: "unsupported channel", mutate: func(in *CreateInput) { in.Channel = "UNKNOWN" }},
		{name: "blank product", mutate: func(in *CreateInput) { in.ExternalProductID = "" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			repository := &memoryRepository{records: make(map[string]Record)}
			service := NewService(repository, func() string { return "TRK-new" }, time.Now)

			_, err := service.Create(context.Background(), input)

			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
			if repository.saveCalls != 0 {
				t.Fatalf("save calls = %d, want 0", repository.saveCalls)
			}
		})
	}
}
