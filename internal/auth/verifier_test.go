package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"testing"
	"time"
)

func tokenFixture(t *testing.T) (Verifier, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewVerifier(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), "https://issuer.example", "shlms-api")
	if err != nil {
		t.Fatal(err)
	}
	v.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	return v, key
}
func signedToken(t *testing.T, key *rsa.PrivateKey, header, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(header)
	p, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding.EncodeToString
	signed := enc(h) + "." + enc(p)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + enc(signature)
}
func goodClaims() map[string]any {
	return map[string]any{"iss": "https://issuer.example", "aud": "shlms-api", "sub": "user-1", "exp": 1_800_000_600, "nbf": 1_799_999_000}
}
func goodHeader() map[string]any { return map[string]any{"alg": "RS256", "typ": "at+jwt"} }

func TestVerifierAcceptsOnlyExpectedAccessToken(t *testing.T) {
	v, key := tokenFixture(t)
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+signedToken(t, key, goodHeader(), goodClaims()))
	userID, err := v.ResolveUserID(r)
	if err != nil || userID != "user-1" {
		t.Fatal(userID, err)
	}
	claims := goodClaims()
	claims["aud"] = []string{"another-api", "shlms-api"}
	r.Header.Set("Authorization", "Bearer "+signedToken(t, key, goodHeader(), claims))
	if _, err = v.ResolveUserID(r); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierRejectsForgedOrWrongPurposeTokens(t *testing.T) {
	v, key := tokenFixture(t)
	_, other := tokenFixture(t)
	cases := map[string]func(map[string]any, map[string]any){
		"wrong algorithm":            func(h, c map[string]any) { h["alg"] = "HS256" },
		"identity token":             func(h, c map[string]any) { h["typ"] = "JWT" },
		"critical header":            func(h, c map[string]any) { h["crit"] = []string{"unknown"} },
		"wrong issuer":               func(h, c map[string]any) { c["iss"] = "https://evil.example" },
		"wrong audience":             func(h, c map[string]any) { c["aud"] = "different-api" },
		"expired":                    func(h, c map[string]any) { c["exp"] = 1_800_000_000 },
		"future":                     func(h, c map[string]any) { c["nbf"] = 1_800_000_001 },
		"blank subject":              func(h, c map[string]any) { c["sub"] = " " },
		"role cannot change subject": func(h, c map[string]any) { c["sub"] = "\nadmin" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h, c := goodHeader(), goodClaims()
			mutate(h, c)
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", "Bearer "+signedToken(t, key, h, c))
			if _, err := v.ResolveUserID(r); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+signedToken(t, other, goodHeader(), goodClaims()))
	if _, err := v.ResolveUserID(r); err == nil {
		t.Fatal("wrong signing key accepted")
	}
	r.Header.Set("Authorization", "Bearer "+signedToken(t, key, goodHeader(), goodClaims())+".extra")
	if _, err := v.ResolveUserID(r); err == nil {
		t.Fatal("extra token segment accepted")
	}
}

func TestVerifierRequiresCompleteConfiguration(t *testing.T) {
	if _, err := NewVerifier(nil, "issuer", "aud"); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := NewVerifier([]byte("bad"), "issuer", "aud"); err == nil {
		t.Fatal("malformed key accepted")
	}
	v, key := tokenFixture(t)
	public, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public})
	if _, err := NewVerifier(pemBytes, "", "aud"); err == nil {
		t.Fatal("missing issuer accepted")
	}
	if _, err := NewVerifier(pemBytes, "issuer", ""); err == nil {
		t.Fatal("missing audience accepted")
	}
	if _, err := v.ResolveUserID(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Fatal("missing bearer accepted")
	}
}
