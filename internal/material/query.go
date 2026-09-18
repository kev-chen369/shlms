package material

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

// OwnerID must come from trusted identity resolution, not a query parameter.
// A cursor is a pagination boundary, never an authorization credential.
type Query struct {
	OwnerID string
	Scope   Context
	Limit   int
	Cursor  string
}

type queryCursor struct {
	Version int     `json:"version"`
	OwnerID string  `json:"ownerId"`
	Scope   Context `json:"scope"`
	AfterID string  `json:"afterId"`
}

func validID(id string) bool {
	return uuid.MatchString(id) && id != "00000000-0000-0000-0000-000000000000"
}
func validQuery(q Query) bool {
	return validID(q.OwnerID) && platformType(q.Scope.Platform, q.Scope.Type) && terminal(q.Scope.Terminal) && text(q.Scope.CityCode, 32, true) && text(q.Scope.Business, 40, true) && q.Limit >= 1 && q.Limit <= 100
}

// ParseQuery validates the exact owner/filter scope before a repository query.
// The repository must still bind owner and all filters independently in SQL.
func ParseQuery(q Query) (string, error) {
	if !validQuery(q) {
		return "", ErrInvalid
	}
	if q.Cursor == "" {
		return "", nil
	}
	if len(q.Cursor) > 1024 {
		return "", ErrInvalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(q.Cursor)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != q.Cursor {
		return "", ErrInvalid
	}
	var cursor queryCursor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cursor) != nil {
		return "", ErrInvalid
	}
	canonical, err := json.Marshal(cursor)
	// Exact canonical JSON also rejects duplicate keys, whitespace and trailing data.
	if err != nil || !bytes.Equal(raw, canonical) || cursor.Version != 1 || cursor.OwnerID != q.OwnerID || cursor.Scope != q.Scope || !validID(cursor.AfterID) {
		return "", ErrInvalid
	}
	return cursor.AfterID, nil
}

func EncodeCursor(q Query, afterID string) (string, error) {
	if _, err := ParseQuery(q); err != nil || !validID(afterID) {
		return "", ErrInvalid
	}
	raw, err := json.Marshal(queryCursor{Version: 1, OwnerID: q.OwnerID, Scope: q.Scope, AfterID: afterID})
	if err != nil {
		return "", ErrInvalid
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
