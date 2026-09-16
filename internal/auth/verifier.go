package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("invalid access token")

type Verifier struct {
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
	now       func() time.Time
}

// NewVerifier accepts only a configured RSA public key. The identity provider
// must issue RS256 access tokens with typ=at+jwt for this API audience.
func NewVerifier(pemBytes []byte, issuer, audience string) (Verifier, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return Verifier{}, errors.New("token issuer and audience are required")
	}
	block, rest := pem.Decode(pemBytes)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 || block.Type != "PUBLIC KEY" {
		return Verifier{}, errors.New("PEM RSA public key required")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return Verifier{}, err
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok || rsaKey.N.BitLen() < 2048 {
		return Verifier{}, errors.New("RSA public key must be at least 2048 bits")
	}
	return Verifier{publicKey: rsaKey, issuer: issuer, audience: audience, now: time.Now}, nil
}

type jwtHeader struct {
	Algorithm string   `json:"alg"`
	Type      string   `json:"typ"`
	Critical  []string `json:"crit"`
}
type audienceClaim []string

func (a *audienceClaim) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

type jwtClaims struct {
	Issuer    string        `json:"iss"`
	Audience  audienceClaim `json:"aud"`
	Subject   string        `json:"sub"`
	Expires   int64         `json:"exp"`
	NotBefore int64         `json:"nbf"`
}

func (v Verifier) ResolveUserID(r *http.Request) (string, error) {
	if v.publicKey == nil || v.now == nil {
		return "", ErrUnauthorized
	}
	header := r.Header.Get("Authorization")
	if len(header) > 8192 {
		return "", ErrUnauthorized
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", ErrUnauthorized
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrUnauthorized
	}
	decode := func(part string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(part) }
	headerJSON, err := decode(parts[0])
	if err != nil {
		return "", ErrUnauthorized
	}
	payloadJSON, err := decode(parts[1])
	if err != nil {
		return "", ErrUnauthorized
	}
	signature, err := decode(parts[2])
	if err != nil {
		return "", ErrUnauthorized
	}
	var h jwtHeader
	if err = json.Unmarshal(headerJSON, &h); err != nil || h.Algorithm != "RS256" || h.Type != "at+jwt" || len(h.Critical) != 0 {
		return "", ErrUnauthorized
	}
	signed := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signed))
	if err = rsa.VerifyPKCS1v15(v.publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return "", ErrUnauthorized
	}
	var c jwtClaims
	if err = json.Unmarshal(payloadJSON, &c); err != nil {
		return "", ErrUnauthorized
	}
	if c.Issuer != v.issuer || c.Expires <= v.now().Unix() || c.NotBefore > v.now().Unix() || !validSubject(c.Subject) {
		return "", ErrUnauthorized
	}
	allowed := false
	for _, audience := range c.Audience {
		if audience == v.audience {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", ErrUnauthorized
	}
	return c.Subject, nil
}

func validSubject(subject string) bool {
	if subject == "" || len(subject) > 256 || strings.TrimSpace(subject) != subject {
		return false
	}
	for _, r := range subject {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}
