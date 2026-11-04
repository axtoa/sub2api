package upstreambalance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

type AccountSource interface {
	GetByID(context.Context, int64) (*service.Account, error)
}
type Probe struct {
	Config *config.Config
	Send   func(*http.Request, string) (*http.Response, error)
}

// Only stable reason codes leave this package. Never return a request URL, key,
// raw upstream body or transport error (which can contain proxy credentials).
func probeError(code string) error { return errors.New(code) }

// Upstream JSON numbers may contain more than six decimals or exponent
// notation. Normalize at the boundary with decimal arithmetic, not float64.
var upstreamQuotaPattern = regexp.MustCompile(`^-?[0-9]{1,24}(\.[0-9]{1,30})?([eE][+-]?[0-9]{1,2})?$`)

var upstreamNumberPattern = regexp.MustCompile(`^-?[0-9]{1,12}(\.[0-9]{1,30})?([eE][+-]?[0-9]{1,2})?$`)

func upstreamAmount(s string) (string, error) {
	if !upstreamNumberPattern.MatchString(s) {
		return "", probeError("invalid_response")
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || new(big.Rat).Abs(r).Cmp(big.NewRat(1000000000, 1)) >= 0 {
		return "", probeError("invalid_response")
	}
	return r.FloatString(6), nil
}
func probeURL(raw string, cfg *config.Config) (string, error) {
	return balanceProbeURL(raw, cfg, "sub2api")
}
func balanceProbeURL(raw string, cfg *config.Config, mode string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" {
		return "", probeError("invalid_url")
	}
	// New balance queries always require TLS, even when legacy forwarding allows HTTP.
	if u.Scheme != "https" {
		return "", probeError("invalid_url")
	}
	base := strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1")
	u.Path = base + "/v1/usage"
	if mode == "newapi" {
		u.Path = base + "/api/usage/token/"
	}
	if mode == "newapi_account" {
		u.Path = base + "/api/user/self"
	}
	u.RawPath = ""
	opts := urlvalidator.ValidationOptions{AllowPrivate: false}
	if cfg != nil && cfg.Security.URLAllowlist.Enabled {
		opts.AllowedHosts = cfg.Security.URLAllowlist.UpstreamHosts
		opts.RequireAllowlist = true
		opts.AllowPrivate = cfg.Security.URLAllowlist.AllowPrivateHosts
	}
	normalized, e := urlvalidator.ValidateHTTPSURL(u.String(), opts)
	if e != nil {
		return "", probeError("url_rejected")
	}
	if mode == "newapi" {
		normalized = strings.TrimRight(normalized, "/") + "/"
	}
	return normalized, nil
}
func (p *Probe) Query(ctx context.Context, a *service.Account) (*Observation, error) {
	return p.QueryWallet(ctx, a, Config{Mode: "sub2api"})
}
func (p *Probe) QueryWallet(ctx context.Context, a *service.Account, wallet Config) (*Observation, error) {
	return p.queryWallet(ctx, a, wallet, "")
}
func (p *Probe) queryWallet(ctx context.Context, a *service.Account, wallet Config, accessToken string) (*Observation, error) {
	accountAuth := wallet.Mode == "newapi" && wallet.NewAPIAuth == "account"
	if a == nil || a.Type != service.AccountTypeAPIKey || (!accountAuth && a.GetCredential("api_key") == "") {
		return nil, probeError("invalid_account")
	}
	mode, token := wallet.Mode, a.GetCredential("api_key")
	if accountAuth {
		if accessToken == "" {
			return nil, probeError("credential_missing")
		}
		mode, token = "newapi_account", accessToken
	}
	target, e := balanceProbeURL(a.GetCredential("base_url"), p.Config, mode)
	if e != nil {
		return nil, e
	}
	proxy := ""
	if a.ProxyID != nil {
		if a.Proxy == nil || a.Proxy.ID != *a.ProxyID {
			return nil, probeError("proxy_unavailable")
		}
		proxy = a.Proxy.URL()
	}
	req, e := http.NewRequestWithContext(service.WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodGet, target, nil)
	if e != nil {
		return nil, probeError("invalid_url")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if accountAuth && wallet.NewAPIUserID > 0 {
		req.Header.Set("New-Api-User", strconv.FormatInt(wallet.NewAPIUserID, 10))
	}
	req.Header.Set("Accept", "application/json")
	// No model requests, header override scripts, rate sync, account extra writes or stop-scheduling hooks.
	send := p.Send
	if send == nil {
		send = func(req *http.Request, proxy string) (*http.Response, error) {
			allowPrivate := p.Config != nil && p.Config.Security.URLAllowlist.Enabled && p.Config.Security.URLAllowlist.AllowPrivateHosts
			client, err := httpclient.GetClient(httpclient.Options{ProxyURL: proxy, Timeout: 12 * time.Second, ResponseHeaderTimeout: 10 * time.Second, ValidateResolvedIP: true, AllowPrivateHosts: allowPrivate, MaxIdleConns: 8, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 3})
			if err != nil {
				return nil, err
			}
			independent := *client
			independent.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			return independent.Do(req)
		}
	}
	resp, e := send(req, proxy)
	if e != nil {
		return nil, probeError("request_failed")
	}
	if resp == nil || resp.Body == nil {
		return nil, probeError("invalid_response")
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case 404, 405:
		return nil, probeError("unsupported")
	case 401, 403:
		return nil, probeError("authentication_failed")
	case 429:
		return nil, probeError("rate_limited")
	}
	if resp.StatusCode != 200 {
		return nil, probeError("http_error")
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, (512<<10)+1))
	if e != nil || len(body) > 512<<10 {
		return nil, probeError("invalid_response")
	}
	if wallet.Mode == "newapi" {
		if accountAuth {
			return parseNewAPIAccountObservation(body, wallet.QuotaPerUnit, wallet.NewAPIUserID, time.Now().UTC())
		}
		return parseNewAPIObservation(body, wallet.QuotaPerUnit, time.Now().UTC())
	}
	return parseObservation(body, time.Now().UTC())
}
func parseObservation(body []byte, now time.Time) (*Observation, error) {
	var v struct {
		Mode         string          `json:"mode"`
		Unit         string          `json:"unit"`
		Balance      json.Number     `json:"balance"`
		Remaining    json.Number     `json:"remaining"`
		Subscription json.RawMessage `json:"subscription"`
		PlanName     string          `json:"planName"`
		Quota        *struct {
			Remaining json.Number `json:"remaining"`
			Unit      string      `json:"unit"`
		} `json:"quota"`
	}
	if json.Unmarshal(body, &v) != nil {
		return nil, probeError("invalid_response")
	}
	value, scope, unit := "", "", v.Unit
	switch v.Mode {
	case "quota_limited":
		if v.Quota == nil {
			return nil, probeError("no_balance")
		}
		value, scope = v.Quota.Remaining.String(), "key_quota"
		if v.Quota.Unit != "" {
			unit = v.Quota.Unit
		}
	case "unrestricted":
		if len(v.Subscription) > 0 && string(v.Subscription) != "null" {
			var plan map[string]json.RawMessage
			if json.Unmarshal(v.Subscription, &plan) != nil || (plan["daily_limit_usd"] == nil && plan["weekly_limit_usd"] == nil && plan["monthly_limit_usd"] == nil && plan["expires_at"] == nil) {
				return nil, probeError("invalid_response")
			}
			value, scope = v.Remaining.String(), "subscription"
		} else if v.Balance != "" {
			value, scope = v.Balance.String(), "wallet"
		} else {
			return nil, probeError("no_balance")
		}
	default:
		return nil, probeError("unsupported")
	}
	unit = strings.ToUpper(unit)
	if !currency(unit) {
		return nil, probeError("unknown_currency")
	}
	n, e := upstreamAmount(value)
	if e != nil {
		return nil, probeError("invalid_response")
	}
	return &Observation{Amount: n, Currency: unit, Scope: scope, At: now}, nil
}

// New API token quota is not wallet money. User quota is opt-in per API key.
func parseNewAPIObservation(body []byte, quotaPerUnit int64, now time.Time) (*Observation, error) {
	var v struct {
		Code    *bool `json:"code"`
		Success *bool `json:"success"`
		Data    *struct {
			Available json.Number `json:"total_available"`
			Unlimited bool        `json:"unlimited_quota"`
			UserQuota *struct {
				Available json.Number `json:"total_available"`
			} `json:"user_quota"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &v) != nil || v.Data == nil || (v.Code == nil && v.Success == nil) || (v.Code != nil && !*v.Code) || (v.Success != nil && !*v.Success) {
		return nil, probeError("invalid_response")
	}
	value, scope := v.Data.Available.String(), "key_quota"
	if v.Data.UserQuota != nil {
		value, scope = v.Data.UserQuota.Available.String(), "wallet"
	} else if v.Data.Unlimited {
		return nil, probeError("wallet_quota_unavailable")
	}
	if value == "" || !upstreamQuotaPattern.MatchString(value) {
		return nil, probeError("invalid_response")
	}
	return newAPIQuotaObservation(value, scope, quotaPerUnit, now)
}

func parseNewAPIAccountObservation(body []byte, ratio, userID int64, now time.Time) (*Observation, error) {
	var v struct {
		Success *bool `json:"success"`
		Data    *struct {
			ID    int64       `json:"id"`
			Quota json.Number `json:"quota"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &v) != nil || v.Success == nil {
		return nil, probeError("invalid_response")
	}
	if !*v.Success {
		return nil, probeError("authentication_failed")
	}
	if v.Data == nil {
		return nil, probeError("invalid_response")
	}
	if userID > 0 && v.Data.ID != userID {
		return nil, probeError("credential_user_mismatch")
	}
	return newAPIQuotaObservation(v.Data.Quota.String(), "wallet", ratio, now)
}

func newAPIQuotaObservation(value, scope string, quotaPerUnit int64, now time.Time) (*Observation, error) {
	if value == "" || !upstreamQuotaPattern.MatchString(value) {
		return nil, probeError("invalid_response")
	}
	if quotaPerUnit == 0 {
		quotaPerUnit = 500000
	}
	if quotaPerUnit < 1 || quotaPerUnit > 1000000000000 {
		return nil, probeError("invalid_response")
	}
	raw, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, probeError("invalid_response")
	}
	converted := raw.Quo(raw, big.NewRat(quotaPerUnit, 1)).FloatString(6)
	amount, err := upstreamAmount(converted)
	if err != nil {
		return nil, err
	}
	return &Observation{Amount: amount, Currency: "USD", Scope: scope, At: now}, nil
}
