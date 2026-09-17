// Package order keeps channel order evidence separate from attribution and money.
package order

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

var (
	ErrInvalid        = errors.New("invalid order evidence")
	ErrConflict       = errors.New("channel event id conflicts with existing evidence")
	ErrNotFound       = errors.New("order evidence not found")
	ErrKeyUnavailable = errors.New("order evidence key unavailable")
)

type RawEvent struct {
	Channel, EventID, ExternalOrderID, EventType string
	OccurredAt                                   time.Time
	Payload                                      []byte
}

type Evidence struct {
	ID, Channel, EventID, ExternalOrderID, EventType string
	OccurredAt, ReceivedAt                           time.Time
	Payload                                          []byte
}

type Store struct {
	db         *sql.DB
	keyVersion string
	key        []byte
}

// The key must come from external secret management. No HTTP endpoint is
// registered for this store; only a verified channel adapter may call Save.
func NewStore(db *sql.DB, keyVersion string, key []byte) (Store, error) {
	if db == nil || !validText(keyVersion, 64) || len(key) != 32 {
		return Store{}, ErrInvalid
	}
	return Store{db: db, keyVersion: keyVersion, key: append([]byte(nil), key...)}, nil
}

func validText(value string, max int) bool {
	return len(value) > 0 && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func validEvent(in RawEvent) bool {
	if in.Channel != "JD" && in.Channel != "TB" && in.Channel != "MT" {
		return false
	}
	if in.EventType != "ORDER" && in.EventType != "REFUND" {
		return false
	}
	if !validText(in.EventID, 128) || !validText(in.ExternalOrderID, 128) || in.OccurredAt.IsZero() ||
		len(in.Payload) == 0 || len(in.Payload) > 64<<10 {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(in.Payload, &object) == nil && object != nil
}

func associatedData(in RawEvent, hash [32]byte) []byte {
	return []byte(strings.Join([]string{in.Channel, in.EventID, in.ExternalOrderID, in.EventType,
		in.OccurredAt.UTC().Format(time.RFC3339Nano), hex.EncodeToString(hash[:])}, "\x00"))
}

func (s Store) Save(ctx context.Context, in RawEvent) (Evidence, bool, error) {
	if s.db == nil || len(s.key) != 32 || !validEvent(in) {
		return Evidence{}, false, ErrInvalid
	}
	in.OccurredAt = in.OccurredAt.UTC().Truncate(time.Microsecond)
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return Evidence{}, false, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Evidence{}, false, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Evidence{}, false, err
	}
	hash := sha256.Sum256(in.Payload)
	ciphertext := aead.Seal(nil, nonce, in.Payload, associatedData(in, hash))
	idBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, idBytes); err != nil {
		return Evidence{}, false, err
	}
	id := hex.EncodeToString(idBytes)
	result, err := s.db.ExecContext(ctx, `INSERT INTO order_raw_events
		(id,channel,event_id,external_order_id,event_type,occurred_at,payload_sha256,payload_nonce,payload_ciphertext,key_version)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(channel,event_id) DO NOTHING`,
		id, in.Channel, in.EventID, in.ExternalOrderID, in.EventType, in.OccurredAt, hash[:], nonce, ciphertext, s.keyVersion)
	if err != nil {
		return Evidence{}, false, err
	}
	created, err := result.RowsAffected()
	if err != nil {
		return Evidence{}, false, err
	}
	var stored Evidence
	var storedHash []byte
	err = s.db.QueryRowContext(ctx, `SELECT id,channel,event_id,external_order_id,event_type,occurred_at,received_at,payload_sha256
		FROM order_raw_events WHERE channel=$1 AND event_id=$2`, in.Channel, in.EventID).
		Scan(&stored.ID, &stored.Channel, &stored.EventID, &stored.ExternalOrderID, &stored.EventType,
			&stored.OccurredAt, &stored.ReceivedAt, &storedHash)
	if err != nil {
		return Evidence{}, false, err
	}
	if stored.ExternalOrderID != in.ExternalOrderID || stored.EventType != in.EventType ||
		!stored.OccurredAt.Equal(in.OccurredAt) || !equalHash(storedHash, hash[:]) {
		return Evidence{}, false, ErrConflict
	}
	return stored, created == 1, nil
}

func equalHash(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var mismatch byte
	for i := range a {
		mismatch |= a[i] ^ b[i]
	}
	return mismatch == 0
}

// Read is an internal processing boundary. Authentication and channel
// provenance must be checked by the caller before consuming an event.
func (s Store) Read(ctx context.Context, id string) (Evidence, error) {
	if s.db == nil || len(s.key) != 32 || !validText(id, 128) {
		return Evidence{}, ErrInvalid
	}
	var e Evidence
	var hash, nonce, ciphertext []byte
	var keyVersion string
	err := s.db.QueryRowContext(ctx, `SELECT id,channel,event_id,external_order_id,event_type,occurred_at,received_at,
		payload_sha256,payload_nonce,payload_ciphertext,key_version FROM order_raw_events WHERE id=$1`, id).
		Scan(&e.ID, &e.Channel, &e.EventID, &e.ExternalOrderID, &e.EventType, &e.OccurredAt, &e.ReceivedAt,
			&hash, &nonce, &ciphertext, &keyVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return Evidence{}, ErrNotFound
	}
	if err != nil {
		return Evidence{}, err
	}
	if keyVersion != s.keyVersion {
		return Evidence{}, ErrKeyUnavailable
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return Evidence{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Evidence{}, err
	}
	var digest [32]byte
	if len(hash) != 32 || len(nonce) != aead.NonceSize() {
		return Evidence{}, ErrInvalid
	}
	copy(digest[:], hash)
	e.Payload, err = aead.Open(nil, nonce, ciphertext, associatedData(RawEvent{
		Channel: e.Channel, EventID: e.EventID, ExternalOrderID: e.ExternalOrderID,
		EventType: e.EventType, OccurredAt: e.OccurredAt,
	}, digest))
	if err != nil || sha256.Sum256(e.Payload) != digest {
		return Evidence{}, ErrInvalid
	}
	return e, nil
}
