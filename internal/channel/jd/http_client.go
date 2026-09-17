package jd

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	commonPromotionMethod = "jd.union.open.promotion.common.get"
	productionGateway     = "https://api.jd.com/routerjson"
)

var (
	ErrHTTPConfig       = errors.New("JD promotion HTTP client is not configured")
	ErrHTTPInput        = errors.New("JD promotion request is invalid")
	ErrProviderRejected = errors.New("JD promotion request was rejected")
	ErrProviderUnknown  = errors.New("JD promotion outcome is unknown")
	subUnionPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
)

// HTTPConfig is supplied by a server-side secret manager. The authorization
// key shown under "My API" is not an AppKey or AppSecret and must not be used.
type HTTPConfig struct {
	AppKey, AppSecret, SiteID string
	Scene2Approved            bool
	SubUnionApproved          bool
}

type HTTPClient struct {
	config   HTTPConfig
	endpoint string
	client   *http.Client
	now      func() time.Time
}

// NewHTTPClient cannot enable the privileged product scene implicitly.
// No production route currently instantiates this client.
func NewHTTPClient(config HTTPConfig) (*HTTPClient, error) {
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     &http.Transport{Proxy: nil},
	}
	return newHTTPClient(config, productionGateway, client, time.Now)
}

func newHTTPClient(config HTTPConfig, endpoint string, client *http.Client, now func() time.Time) (*HTTPClient, error) {
	if strings.TrimSpace(config.AppKey) == "" || strings.TrimSpace(config.AppSecret) == "" ||
		!skuPattern.MatchString(config.SiteID) || !config.Scene2Approved ||
		client == nil || now == nil || endpoint == "" {
		return nil, ErrHTTPConfig
	}
	return &HTTPClient{config: config, endpoint: endpoint, client: client, now: now}, nil
}

type productCodeReq struct {
	MaterialID string `json:"materialId"`
	SiteID     string `json:"siteId"`
	PositionID uint64 `json:"positionId,omitempty"`
	SceneID    int    `json:"sceneId"`
	SubUnionID string `json:"subUnionId,omitempty"`
}

func (c *HTTPClient) GeneratePromotionLink(ctx context.Context, input ClientRequest) (ClientResponse, error) {
	if c == nil {
		return ClientResponse{}, ErrHTTPConfig
	}
	material, err := ProductMaterial(input.MaterialID)
	if err != nil || material != input.MaterialID || input.SubUnionID != "" &&
		(!c.config.SubUnionApproved || !subUnionPattern.MatchString(input.SubUnionID)) {
		return ClientResponse{}, ErrHTTPInput
	}
	var position uint64
	if input.PositionID != "" {
		position, err = strconv.ParseUint(input.PositionID, 10, 64)
		if err != nil || position == 0 {
			return ClientResponse{}, ErrHTTPInput
		}
	}
	if err := ctx.Err(); err != nil {
		return ClientResponse{}, err
	}
	payload, err := json.Marshal(struct {
		PromotionCodeReq productCodeReq `json:"promotionCodeReq"`
	}{productCodeReq{MaterialID: material, SiteID: c.config.SiteID, PositionID: position, SceneID: 2, SubUnionID: input.SubUnionID}})
	if err != nil {
		return ClientResponse{}, ErrHTTPInput
	}
	params := url.Values{
		"360buy_param_json": {string(payload)},
		"app_key":           {c.config.AppKey},
		"format":            {"json"},
		"method":            {commonPromotionMethod},
		"sign_method":       {"md5"},
		"timestamp":         {c.now().In(time.FixedZone("GMT+8", 8*3600)).Format("2006-01-02 15:04:05")},
		"v":                 {"1.0"},
	}
	params.Set("sign", signParams(params, c.config.AppSecret))
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return ClientResponse{}, ErrHTTPConfig
	}
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ClientResponse{}, ErrHTTPConfig
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ClientResponse{}, ErrProviderUnknown
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ClientResponse{}, ErrProviderUnknown
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024+1))
	if err != nil || len(body) > 64*1024 {
		return ClientResponse{}, ErrProviderUnknown
	}
	var envelope struct {
		Result struct {
			GetResult json.RawMessage `json:"getResult"`
		} `json:"jd_union_open_promotion_common_get_responce"`
		Error json.RawMessage `json:"error_response"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Error) != 0 || len(envelope.Result.GetResult) == 0 {
		return ClientResponse{}, ErrProviderUnknown
	}
	var result struct {
		Code json.RawMessage `json:"code"`
		Data struct {
			ClickURL string `json:"clickURL"`
		} `json:"data"`
	}
	inner := envelope.Result.GetResult
	if len(inner) != 0 && inner[0] == '"' {
		var value string
		if json.Unmarshal(inner, &value) != nil {
			return ClientResponse{}, ErrProviderUnknown
		}
		inner = []byte(value)
	}
	if json.Unmarshal(inner, &result) != nil {
		return ClientResponse{}, ErrProviderUnknown
	}
	if code := strings.Trim(string(result.Code), `"`); code != "200" {
		if code == "" {
			return ClientResponse{}, ErrProviderUnknown
		}
		return ClientResponse{}, ErrProviderRejected
	}
	if !validPromotionURL(result.Data.ClickURL) {
		return ClientResponse{}, ErrProviderUnknown
	}
	return ClientResponse{URL: result.Data.ClickURL}, nil
}

func signParams(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "sign" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(secret)
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString(params.Get(key))
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func validPromotionURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "union-click.jd.com" &&
		u.User == nil && u.Fragment == "" && u.RawFragment == "" &&
		!strings.Contains(raw, "#") && len(raw) <= 4096
}
