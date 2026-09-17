package coupon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
)

var (
	ErrOutboundInvalid     = errors.New("invalid outbound request")
	ErrOutboundUnavailable = errors.New("outbound target unavailable")
	ErrOutboundConflict    = errors.New("outbound idempotency conflict")
)

type OutboundInput struct {
	OwnerKey, CouponID, CityCode, Business, Terminal, EntryPoint, IdempotencyKey string
}

type OutboundTarget struct {
	URL string
	// EvidenceRef identifies the approved channel response without storing its raw token.
	EvidenceRef string
}

type OutboundResolver interface {
	ResolveOutbound(context.Context, Item, OutboundInput) (OutboundTarget, error)
}

type OutboundResult struct {
	ID, CouponID, Platform, TargetURL, TargetHost string
	CreatedAt                                     time.Time
}

// OutboundService is not wired in production until an approved channel resolver
// and exact host allowlist are configured. A prepared link is never a claim receipt.
type OutboundService struct {
	Catalog      Catalog
	DB           *sql.DB
	Resolver     OutboundResolver
	AllowedHosts map[string]map[string]bool
}

func (s OutboundService) Prepare(ctx context.Context, in OutboundInput) (OutboundResult, error) {
	if s.DB == nil || s.Resolver == nil || !validID(in.OwnerKey) || !validID(in.CouponID) ||
		!validContext(in.CityCode, in.Business) || !validClaimKey(in.IdempotencyKey) ||
		!validText(in.EntryPoint, 64) || (in.Terminal != "H5" && in.Terminal != "APP" && in.Terminal != "WECHAT") {
		return OutboundResult{}, ErrOutboundInvalid
	}
	item, err := s.Catalog.Get(ctx, in.CouponID, in.CityCode, in.Business)
	if errors.Is(err, ErrNotFound) {
		return OutboundResult{}, ErrNotFound
	}
	if err != nil {
		return OutboundResult{}, err
	}
	if item.ClaimMode == "IN_SITE_VERIFIED" {
		return OutboundResult{}, ErrOutboundInvalid
	}
	// The resolver alone may supply a target; no client URL is accepted.
	target, err := s.Resolver.ResolveOutbound(ctx, item, in)
	if err != nil {
		return OutboundResult{}, ErrOutboundUnavailable
	}
	u, err := url.Parse(target.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" ||
		u.Port() != "" || net.ParseIP(u.Hostname()) != nil || strings.EqualFold(u.Hostname(), "localhost") ||
		!s.AllowedHosts[item.Platform][strings.ToLower(u.Hostname())] ||
		!validText(target.EvidenceRef, 256) {
		return OutboundResult{}, ErrOutboundUnavailable
	}
	fingerprint := sha256.Sum256([]byte(strings.Join([]string{in.OwnerKey, in.CouponID, in.CityCode, in.Business, in.Terminal, in.EntryPoint}, "\x00")))
	fp := hex.EncodeToString(fingerprint[:])
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return OutboundResult{}, err
	}
	id := hex.EncodeToString(idBytes)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO coupon_outbounds
		(id,owner_key,coupon_id,idempotency_key,request_fingerprint,platform,target_host,evidence_ref,terminal,entry_point)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(owner_key,idempotency_key) DO NOTHING`,
		id, in.OwnerKey, in.CouponID, in.IdempotencyKey, fp, item.Platform, strings.ToLower(u.Hostname()), target.EvidenceRef, in.Terminal, in.EntryPoint)
	if err != nil {
		return OutboundResult{}, err
	}
	var result OutboundResult
	var storedFP, terminal, entry string
	err = s.DB.QueryRowContext(ctx, `SELECT id,coupon_id,platform,target_host,request_fingerprint,terminal,entry_point,created_at
		FROM coupon_outbounds WHERE owner_key=$1 AND idempotency_key=$2`, in.OwnerKey, in.IdempotencyKey).
		Scan(&result.ID, &result.CouponID, &result.Platform, &result.TargetHost, &storedFP, &terminal, &entry, &result.CreatedAt)
	if err != nil {
		return OutboundResult{}, err
	}
	if storedFP != fp || result.CouponID != in.CouponID || terminal != in.Terminal || entry != in.EntryPoint || result.TargetHost != strings.ToLower(u.Hostname()) {
		return OutboundResult{}, ErrOutboundConflict
	}
	result.TargetURL = target.URL
	return result, nil
}
