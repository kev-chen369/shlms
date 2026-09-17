package jd

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var ErrInvalidMaterial = errors.New("JD product must be a decimal SKU or a supported HTTPS product URL")

// Keep IDs as strings: JD product identifiers can exceed JavaScript's safe
// integer range. Normalization does not fetch the URL or create a promotion.
var skuPattern = regexp.MustCompile(`^[1-9][0-9]{0,31}$`)
var productPathPattern = regexp.MustCompile(`^/([1-9][0-9]{0,31})\.html$`)

// ProductMaterial accepts direct desktop JD product links and plain SKUs.
// Short links, coupon links, activity pages and arbitrary shared text require
// their own verified resolver; they are not fetched or guessed here.
func ProductMaterial(input string) (string, error) {
	input = strings.TrimSpace(input)
	if skuPattern.MatchString(input) {
		return "https://item.jd.com/" + input + ".html", nil
	}
	if len(input) > 2048 || strings.ContainsAny(input, "\r\n\t\\") {
		return "", ErrInvalidMaterial
	}
	u, err := url.Parse(input)
	if err != nil || u.Scheme != "https" || u.User != nil ||
		!strings.EqualFold(u.Host, "item.jd.com") || u.RawQuery != "" ||
		u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || strings.Contains(input, "#") || u.RawPath != "" {
		return "", ErrInvalidMaterial
	}
	match := productPathPattern.FindStringSubmatch(u.Path)
	if match == nil {
		return "", ErrInvalidMaterial
	}
	return "https://item.jd.com/" + match[1] + ".html", nil
}
