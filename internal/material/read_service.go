package material

import (
	stdcontext "context"
	"time"

	"github.com/kev-chen369/shlms/internal/capability"
)

// CatalogBinding is trusted deployment configuration, never a client input or
// a READY grant. The database must independently approve the exact full Key.
type CatalogBinding struct {
	Platform, Type, Terminal, Scene, MediaID string
}

// ReadInput carries verified identity and requested filters/position. There is
// deliberately no media, status, approval evidence or client timestamp field.
type ReadInput struct {
	OwnerID           string
	Scope             Context
	PositionID, Scene string
}

// Valid checks request shape only, never eligibility or channel permission.
func (in ReadInput) Valid() bool {
	return validQuery(Query{OwnerID: in.OwnerID, Scope: in.Scope, Limit: 1}) && text(in.PositionID, 128, false) && text(in.Scene, 80, false)
}

type bindingScope struct{ platform, kind, terminal, scene string }

// ReadService holds a copied, immutable configuration. No binding means no
// catalog access, even when a database contains a READY row for another media.
type ReadService struct {
	repo     Repository
	bindings map[bindingScope]string
	now      func() time.Time
}

func NewReadService(repo Repository, bindings []CatalogBinding) (*ReadService, error) {
	if repo.DB == nil {
		return nil, ErrUnavailable
	}
	s := &ReadService{repo: repo, bindings: make(map[bindingScope]string, len(bindings)), now: time.Now}
	for _, binding := range bindings {
		if !platformType(binding.Platform, binding.Type) || !terminal(binding.Terminal) || !text(binding.Scene, 80, false) || !text(binding.MediaID, 128, false) {
			return nil, ErrInvalid
		}
		scope := bindingScope{binding.Platform, binding.Type, binding.Terminal, binding.Scene}
		if _, exists := s.bindings[scope]; exists {
			return nil, ErrInvalid
		}
		s.bindings[scope] = binding.MediaID
	}
	return s, nil
}

func (s *ReadService) key(ctx stdcontext.Context, in ReadInput) (capability.Key, bool, error) {
	if s == nil || s.repo.DB == nil || s.now == nil {
		return capability.Key{}, false, ErrUnavailable
	}
	if !in.Valid() {
		return capability.Key{}, false, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return capability.Key{}, false, err
	}
	media, configured := s.bindings[bindingScope{in.Scope.Platform, in.Scope.Type, in.Scope.Terminal, in.Scene}]
	return capability.Key{Platform: in.Scope.Platform, MaterialType: in.Scope.Type, Kind: "CATALOG", MediaID: media, PositionID: in.PositionID, Scene: in.Scene, Terminal: in.Scope.Terminal, CityCode: in.Scope.CityCode, Business: in.Scope.Business}, configured, nil
}

func (s *ReadService) List(ctx stdcontext.Context, in ReadInput, cursor string, limit int) (Page, error) {
	q := Query{OwnerID: in.OwnerID, Scope: in.Scope, Cursor: cursor, Limit: limit}
	if _, err := ParseQuery(q); err != nil {
		return Page{}, ErrInvalid
	}
	key, configured, err := s.key(ctx, in)
	if err != nil {
		return Page{}, err
	}
	if !configured {
		return Page{Items: []Card{}, Capability: capability.Decision{Reason: "UNCONFIGURED"}}, nil
	}
	return s.repo.List(ctx, q, key, s.now())
}

func (s *ReadService) Get(ctx stdcontext.Context, in ReadInput, materialID string) (Detail, error) {
	if !validID(materialID) {
		return Detail{}, ErrInvalid
	}
	key, configured, err := s.key(ctx, in)
	if err != nil {
		return Detail{}, err
	}
	if !configured {
		return Detail{Capability: capability.Decision{Reason: "UNCONFIGURED"}, Availability: Decision{Reason: "CAPABILITY_UNAVAILABLE"}}, nil
	}
	return s.repo.Get(ctx, in.OwnerID, in.Scope, key, materialID, s.now())
}
