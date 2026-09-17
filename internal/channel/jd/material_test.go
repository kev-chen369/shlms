package jd

import (
	"errors"
	"testing"
)

func TestProductMaterial(t *testing.T) {
	for _, input := range []string{"12345678901234567890", " https://ITEM.JD.COM/12345678901234567890.html "} {
		got, err := ProductMaterial(input)
		if err != nil || got != "https://item.jd.com/12345678901234567890.html" {
			t.Fatalf("normalization failed for %q: %q %v", input, got, err)
		}
	}
}

func TestProductMaterialRejectsUnverifiedInputs(t *testing.T) {
	for _, input := range []string{
		"", "0", "01", "-1", "1.2", "1e9", "sku-1", "https://u.jd.com/demo",
		"http://item.jd.com/123.html", "https://item.jd.com.evil.example/123.html",
		"https://item.jd.com@evil.example/123.html", "https://user@item.jd.com/123.html",
		"https://item.jd.com:443/123.html", "https://127.0.0.1/123.html",
		"https://item.jd.com/123.html?redirect=https://example.com", "https://item.jd.com/123.html?",
		"https://item.jd.com/123.html#x", "https://item.jd.com/123.html#", "https://item.jd.com/%31.html",
		"https://item.jd.com/../123.html", "https://item.jd.com/123.html/", "https://item.jd.com/123\n.html",
		"https://item.jd.com\\evil/123.html", "商品链接 https://item.jd.com/123.html",
		"12345678901234567890123456789012345",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := ProductMaterial(input); !errors.Is(err, ErrInvalidMaterial) {
				t.Fatalf("got %v, want invalid material", err)
			}
		})
	}
}
