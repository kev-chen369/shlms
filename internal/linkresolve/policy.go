// Package linkresolve validates and fetches channel URLs under an exact host allowlist.
package linkresolve

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

var (
	ErrURLRejected     = errors.New("referral URL is not permitted")
	ErrAddressRejected = errors.New("referral address is not public")
)

type Policy struct{ hosts map[string]bool }

// NewPolicy requires exact ASCII DNS names configured from approved channel
// documentation. No wildcard, suffix matching or implicit provider hosts.
func NewPolicy(hosts []string) (Policy, error) {
	if len(hosts) == 0 {
		return Policy{}, ErrURLRejected
	}
	p := Policy{hosts: map[string]bool{}}
	for _, host := range hosts {
		if host != strings.ToLower(host) || !validDNSName(host) || net.ParseIP(host) != nil {
			return Policy{}, ErrURLRejected
		}
		p.hosts[host] = true
	}
	return p, nil
}

func validDNSName(host string) bool {
	if len(host) == 0 || len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}

func (p Policy) Validate(raw string) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > 4096 || strings.TrimSpace(raw) != raw {
		return nil, ErrURLRejected
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Fragment != "" {
		return nil, ErrURLRejected
	}
	host := u.Hostname()
	if !validDNSName(host) || !p.hosts[host] || net.ParseIP(host) != nil || u.Port() != "" && u.Port() != "443" {
		return nil, ErrURLRejected
	}
	if _, err = url.ParseQuery(u.RawQuery); err != nil {
		return nil, ErrURLRejected
	}
	return u, nil
}

var blockedIPv4 = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
}
var publicIPv6 = netip.MustParsePrefix("2000::/3")
var blockedIPv6 = []netip.Prefix{
	netip.MustParsePrefix("2001::/32"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2001:10::/28"), netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:2::/48"), netip.MustParsePrefix("2002::/16"),
}

// PublicAddress rejects private, special-use and transition ranges, including
// IPv4-mapped IPv6. The fetcher also pins the accepted IP for dialing.
func PublicAddress(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" {
		return false
	}
	ip := address.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is4() {
		for _, prefix := range blockedIPv4 {
			if prefix.Contains(ip) {
				return false
			}
		}
		return true
	}
	if !publicIPv6.Contains(ip) {
		return false
	}
	for _, prefix := range blockedIPv6 {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
