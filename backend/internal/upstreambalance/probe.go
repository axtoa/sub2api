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
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" {
		return "", probeError("invalid_url")
	}
	// New balance queries always require TLS, even when legacy forwarding allows HTTP.
	if u.Scheme != "https" {
		return "", probeError("invalid_url")
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1") + "/v1/usage"
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
	return normalized, nil
}
func (p *Probe) Query(ctx context.Context, a *service.Account) (*Observation, error) {
	if a == nil || a.Type != service.AccountTypeAPIKey || a.GetCredential("api_key") == "" {
		return nil, probeError("invalid_account")
	}
	target, e := probeURL(a.GetCredential("base_url"), p.Config)
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
	req.Header.Set("Authorization", "Bearer "+a.GetCredential("api_key"))
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
