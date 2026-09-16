package linkresolve

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func testTLS(t *testing.T, handler http.Handler) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "shop.example.com"},
		DNSNames: []string{"shop.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return server, roots
}

func localDialer(t *testing.T, server *httptest.Server) NetworkDialer {
	t.Helper()
	return dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "8.8.8.8:443" {
			t.Fatalf("unsafe address dialed: %s %s", network, address)
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	})
}

func TestFetcherFollowsOnlyApprovedRedirectsWithPinnedDNS(t *testing.T) {
	server, roots := testTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "https://shop.example.com/item?id=1", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("product preview source"))
	}))
	policy, _ := NewPolicy([]string{"shop.example.com"})
	fetcher, _ := NewFetcher(policy)
	fetcher.trustedRoots = roots
	var lookups atomic.Int64
	fetcher.resolver = resolverFunc(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		if network != "ip" || host != "shop.example.com" {
			t.Fatal(network, host)
		}
		lookups.Add(1)
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	fetcher.dialer = localDialer(t, server)
	result, err := fetcher.Fetch(context.Background(), "https://shop.example.com/start")
	if err != nil || result.FinalURL != "https://shop.example.com/item?id=1" || string(result.Body) != "product preview source" || len(result.Hops) != 2 || lookups.Load() != 2 {
		t.Fatal(result, err, lookups.Load())
	}
}

func TestFetcherBlocksDNSRebindingBeforeSecondDial(t *testing.T) {
	server, roots := testTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://shop.example.com/final", http.StatusFound)
	}))
	policy, _ := NewPolicy([]string{"shop.example.com"})
	fetcher, _ := NewFetcher(policy)
	fetcher.trustedRoots = roots
	var calls atomic.Int64
	fetcher.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		if calls.Add(1) == 1 {
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})
	var dials atomic.Int64
	fetcher.dialer = dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		return localDialer(t, server).DialContext(ctx, network, address)
	})
	_, err := fetcher.Fetch(context.Background(), "https://shop.example.com/start")
	if !errors.Is(err, ErrAddressRejected) || calls.Load() != 2 || dials.Load() != 1 {
		t.Fatal(err, calls.Load(), dials.Load())
	}
}

func TestFetcherRejectsRedirectToUnapprovedHost(t *testing.T) {
	server, roots := testTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example.com/metadata", http.StatusFound)
	}))
	policy, _ := NewPolicy([]string{"shop.example.com"})
	fetcher, _ := NewFetcher(policy)
	fetcher.trustedRoots = roots
	fetcher.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	fetcher.dialer = localDialer(t, server)
	_, err := fetcher.Fetch(context.Background(), "https://shop.example.com/start")
	if !errors.Is(err, ErrURLRejected) {
		t.Fatal(err)
	}
}

func TestFetcherLimitsResponseAndRedirects(t *testing.T) {
	server, roots := testTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			_, _ = w.Write([]byte(fmt.Sprintf("%065537d", 0)))
			return
		}
		http.Redirect(w, r, "https://shop.example.com/loop", http.StatusFound)
	}))
	policy, _ := NewPolicy([]string{"shop.example.com"})
	fetcher, _ := NewFetcher(policy)
	fetcher.trustedRoots = roots
	fetcher.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	fetcher.dialer = localDialer(t, server)
	if _, err := fetcher.Fetch(context.Background(), "https://shop.example.com/large"); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatal(err)
	}
	if _, err := fetcher.Fetch(context.Background(), "https://shop.example.com/loop"); !errors.Is(err, ErrTooManyRedirects) {
		t.Fatal(err)
	}
}

func TestFetcherRejectsMixedDNSAnswersBeforeDial(t *testing.T) {
	policy, _ := NewPolicy([]string{"shop.example.com"})
	fetcher, _ := NewFetcher(policy)
	fetcher.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("169.254.169.254")}, nil
	})
	fetcher.dialer = dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("mixed DNS answer reached dialer")
		return nil, nil
	})
	if _, err := fetcher.Fetch(context.Background(), "https://shop.example.com/item"); !errors.Is(err, ErrAddressRejected) {
		t.Fatal(err)
	}
}

func TestFetcherVerifiesTLSHostAfterPinnedDial(t *testing.T) {
	server, roots := testTLS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("item")) }))
	policy, _ := NewPolicy([]string{"go.example.com"})
	fetcher, _ := NewFetcher(policy)
	fetcher.trustedRoots = roots
	fetcher.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	})
	fetcher.dialer = localDialer(t, server)
	if _, err := fetcher.Fetch(context.Background(), "https://go.example.com/item"); err == nil {
		t.Fatal("TLS hostname mismatch accepted")
	}
}
