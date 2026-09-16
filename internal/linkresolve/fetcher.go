package linkresolve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

var (
	ErrTooManyRedirects = errors.New("referral redirect limit exceeded")
	ErrResponseTooLarge = errors.New("referral response exceeds limit")
	ErrUpstreamResponse = errors.New("referral upstream did not return success")
)

type DNSResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type NetworkDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type Hop struct {
	Host    string
	Address netip.Addr
}
type Result struct {
	FinalURL   string
	StatusCode int
	Body       []byte
	Hops       []Hop
}

type Fetcher struct {
	policy       Policy
	resolver     DNSResolver
	dialer       NetworkDialer
	trustedRoots *x509.CertPool
}

func NewFetcher(policy Policy) (Fetcher, error) {
	if len(policy.hosts) == 0 {
		return Fetcher{}, ErrURLRejected
	}
	return Fetcher{policy: policy, resolver: net.DefaultResolver, dialer: &net.Dialer{Timeout: 3 * time.Second}}, nil
}

// Fetch validates every redirect and resolves each connection just before it
// dials. The checked IP is the IP actually dialed, closing DNS rebinding gaps.
func (f Fetcher) Fetch(ctx context.Context, raw string) (Result, error) {
	u, err := f.policy.Validate(raw)
	if err != nil {
		return Result{}, err
	}
	if f.resolver == nil || f.dialer == nil {
		return Result{}, ErrAddressRejected
	}
	var mu sync.Mutex
	hops := []Hop{}
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.trustedRoots},
		TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 3 * time.Second, MaxResponseHeaderBytes: 8 << 10,
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || network != "tcp" || port != "443" || !f.policy.hosts[host] {
			return nil, ErrURLRejected
		}
		ips, err := f.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, ErrAddressRejected
		}
		// A mixed public/private answer is rejected in its entirety.
		for _, ip := range ips {
			if !PublicAddress(ip) {
				return nil, ErrAddressRejected
			}
		}
		var lastErr error
		for _, ip := range ips {
			connection, dialErr := f.dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				mu.Lock()
				hops = append(hops, Hop{Host: host, Address: ip})
				mu.Unlock()
				return connection, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return ErrTooManyRedirects
		}
		if _, err := f.policy.Validate(request.URL.String()); err != nil {
			return err
		}
		request.Header.Del("Authorization")
		request.Header.Del("Cookie")
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, ErrUpstreamResponse
	}
	const maxBody = 64 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return Result{}, err
	}
	if len(body) > maxBody {
		return Result{}, ErrResponseTooLarge
	}
	mu.Lock()
	recorded := append([]Hop(nil), hops...)
	mu.Unlock()
	return Result{FinalURL: response.Request.URL.String(), StatusCode: response.StatusCode, Body: body, Hops: recorded}, nil
}
