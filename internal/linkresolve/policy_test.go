package linkresolve

import (
	"errors"
	"net/netip"
	"testing"
)

func TestPolicyRequiresExactApprovedHTTPSHost(t *testing.T) {
	for _, hosts := range [][]string{nil, {"*.example.com"}, {"LOCALHOST"}, {"localhost"}, {"example.com."}, {"127.0.0.1"}, {"a..example.com"}} {
		if _, err := NewPolicy(hosts); !errors.Is(err, ErrURLRejected) {
			t.Fatal(hosts, err)
		}
	}
	p, err := NewPolicy([]string{"shop.example.com", "go.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://shop.example.com/item?id=1", "https://go.example.com:443/a"} {
		if _, err = p.Validate(raw); err != nil {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{
		"http://shop.example.com/item", "https://evil.example.com/item", "https://shop.example.com.evil.test/item",
		"https://user:secret@shop.example.com/item", "https://shop.example.com:444/item",
		"https://shop.example.com./item", "https://127.0.0.1/item", "https://shop.example.com/item#fragment",
		" https://shop.example.com/item", "https://shop.example.com/item?bad=%zz",
	} {
		if _, err = p.Validate(raw); !errors.Is(err, ErrURLRejected) {
			t.Fatal(raw, err)
		}
	}
}

func TestPublicAddressRejectsSpecialRanges(t *testing.T) {
	for _, raw := range []string{
		"0.0.0.1", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.169.254", "172.20.0.1",
		"192.0.0.1", "192.0.2.1", "192.168.1.1", "198.18.0.1", "198.51.100.1", "203.0.113.1",
		"224.0.0.1", "255.255.255.255", "::1", "fc00::1", "fe80::1", "2001:db8::1", "2002:c0a8:101::1", "::ffff:127.0.0.1",
	} {
		ip := netip.MustParseAddr(raw)
		if PublicAddress(ip) {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		ip := netip.MustParseAddr(raw)
		if !PublicAddress(ip) {
			t.Fatal(raw)
		}
	}
}
