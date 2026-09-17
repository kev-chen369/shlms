package jd

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var testNow = func() time.Time {
	return time.Date(2026, 9, 17, 8, 2, 3, 0, time.UTC)
}

func readyConfig() HTTPConfig {
	return HTTPConfig{AppKey: "app-key", AppSecret: "test-secret", SiteID: "435676", Scene2Approved: true}
}

func TestHTTPClientSignedProductRequest(t *testing.T) {
	const material = "https://item.jd.com/12345678901234567890.html"
	const body = `{"jd_union_open_promotion_common_get_responce":{"getResult":{"code":"200","data":{"clickURL":"https://union-click.jd.com/jdc?demo=1"},"message":"success"}}}`
	var called int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.Method != http.MethodGet || r.URL.Path != "/routerjson" {
			t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		const payload = `{"promotionCodeReq":{"materialId":"https://item.jd.com/12345678901234567890.html","siteId":"435676","positionId":6,"sceneId":2,"subUnionId":"TRK_123-4"}}`
		for key, value := range map[string]string{
			"360buy_param_json": payload, "app_key": "app-key", "format": "json",
			"method": commonPromotionMethod, "sign_method": "md5", "timestamp": "2026-09-17 16:02:03", "v": "1.0",
		} {
			if q.Get(key) != value {
				t.Errorf("%s = %q, want %q", key, q.Get(key), value)
			}
		}
		if len(q) != 8 || strings.Contains(r.URL.RawQuery, "test-secret") {
			t.Error("unexpected parameter or secret in request")
		}
		// Independent protocol vector: literal lexicographic order and unencoded
		// values, with the secret placed only at the ends of the MD5 input.
		unsigned := "test-secret" + "360buy_param_json" + payload + "app_keyapp-key" +
			"formatjson" + "method" + commonPromotionMethod + "sign_methodmd5" +
			"timestamp2026-09-17 16:02:03" + "v1.0" + "test-secret"
		sum := md5.Sum([]byte(unsigned))
		if q.Get("sign") != strings.ToUpper(hex.EncodeToString(sum[:])) {
			t.Error("signature does not match official sorted-field algorithm")
		}
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	config := readyConfig()
	config.SubUnionApproved = true
	client, err := newHTTPClient(config, server.URL+"/routerjson", server.Client(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.GeneratePromotionLink(context.Background(), ClientRequest{
		MaterialID: material, PositionID: "6", SubUnionID: "TRK_123-4",
	})
	if err != nil || got.URL != "https://union-click.jd.com/jdc?demo=1" || called != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", got, err, called)
	}
}

func TestHTTPClientFailsClosedBeforeNetwork(t *testing.T) {
	for _, config := range []HTTPConfig{
		{}, {AppKey: "app-key", AppSecret: "test-secret", SiteID: "435676"},
		{AppKey: "app-key", AppSecret: "test-secret", SiteID: "guide-media", Scene2Approved: true},
	} {
		if _, err := NewHTTPClient(config); !errors.Is(err, ErrHTTPConfig) {
			t.Fatalf("config accepted without approved site/scene: %v", err)
		}
	}
	var calls int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	client, err := newHTTPClient(readyConfig(), server.URL, server.Client(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []ClientRequest{
		{MaterialID: "https://evil.example/123.html"},
		{MaterialID: "123", PositionID: "1"}, // only normalized URLs cross this boundary
		{MaterialID: "https://item.jd.com/123.html", PositionID: "not-numeric"},
		{MaterialID: "https://item.jd.com/123.html", SubUnionID: "TRK-1"},
	} {
		if _, err := client.GeneratePromotionLink(context.Background(), request); !errors.Is(err, ErrHTTPInput) {
			t.Fatalf("invalid request accepted: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.GeneratePromotionLink(ctx, ClientRequest{MaterialID: "https://item.jd.com/123.html"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled call: %v", err)
	}
	if calls != 0 {
		t.Fatalf("made %d calls despite invalid inputs", calls)
	}
}

func TestHTTPClientResponseClassificationAndNoRetry(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"provider denied", `{"jd_union_open_promotion_common_get_responce":{"getResult":{"code":403,"message":"sensitive"}}}`, 200, ErrProviderRejected},
		{"gateway error", `{"error_response":{"code":"invalid","msg":"secret-like-response"}}`, 200, ErrProviderUnknown},
		{"malformed", `not JSON with a private key`, 200, ErrProviderUnknown},
		{"HTTP 500", `private-key-like-content`, 500, ErrProviderUnknown},
		{"plain HTTP link", `{"jd_union_open_promotion_common_get_responce":{"getResult":{"code":"200","data":{"clickURL":"http://union-click.jd.com/jdc?x=1"}}}}`, 200, ErrProviderUnknown},
		{"impostor link", `{"jd_union_open_promotion_common_get_responce":{"getResult":{"code":"200","data":{"clickURL":"https://union-click.jd.com.evil.example/jdc"}}}}`, 200, ErrProviderUnknown},
		{"string result", `{"jd_union_open_promotion_common_get_responce":{"getResult":"{\"code\":200,\"data\":{\"clickURL\":\"https://union-click.jd.com/jdc?ok=1\"}}"}}`, 200, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := newHTTPClient(readyConfig(), server.URL, server.Client(), testNow)
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.GeneratePromotionLink(context.Background(), ClientRequest{MaterialID: "https://item.jd.com/123.html"})
			if !errors.Is(err, tc.want) || calls != 1 {
				t.Fatalf("result=%+v err=%v calls=%d, want %v", result, err, calls, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), "sensitive") {
				t.Fatal("raw provider diagnostic leaked")
			}
		})
	}
}

func TestHTTPClientDoesNotFollowRedirect(t *testing.T) {
	var redirectCalls int
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirectCalls++ }))
	defer other.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", other.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	jdClient, err := newHTTPClient(readyConfig(), server.URL, client, testNow)
	if err != nil {
		t.Fatal(err)
	}
	_, err = jdClient.GeneratePromotionLink(context.Background(), ClientRequest{MaterialID: "https://item.jd.com/123.html"})
	if !errors.Is(err, ErrProviderUnknown) || redirectCalls != 0 {
		t.Fatalf("redirect result=%v target calls=%d", err, redirectCalls)
	}
}

func TestSignParamsIgnoresSignValue(t *testing.T) {
	params := url.Values{"b": {"2"}, "a": {"1"}}
	first := signParams(params, "secret")
	params.Set("sign", "forged")
	if signParams(params, "secret") != first {
		t.Fatal("signature included itself")
	}
}
