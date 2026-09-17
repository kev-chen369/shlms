package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kev-chen369/shlms/internal/channel/jd"
)

func TestCommandFailsClosedWithNoSiteOrCredentials(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"-product", "123456", "-position", "6"}, func(string) string { return "" }, &output)
	if !errors.Is(err, jd.ErrHTTPConfig) || output.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}

func TestCommandRejectsUnapprovedSubUnionAndDoesNotPrintSecrets(t *testing.T) {
	var output bytes.Buffer
	getenv := func(key string) string {
		switch key {
		case "JD_APP_KEY":
			return "test-app-key"
		case "JD_APP_SECRET":
			return "test-only-password-like-secret"
		case "JD_SITE_ID":
			return "435676"
		case "JD_SCENE2_APPROVED":
			return "true"
		default:
			return ""
		}
	}
	for _, args := range [][]string{
		{"-product", "123", "-position", "6", "-tracking", "TRK-1"},
		{"-product", "http://evil.example/123.html", "-position", "6"},
		{"-product", "123", "-position", "not-numeric"},
	} {
		err := run(context.Background(), args, getenv, &output)
		if err == nil || output.Len() != 0 || strings.Contains(err.Error(), "test-only-password") {
			t.Fatalf("failed to reject safely: err=%v output=%q", err, output.String())
		}
	}
}
