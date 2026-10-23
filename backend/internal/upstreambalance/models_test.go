package upstreambalance

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func exampleConfig() Config {
	id := int64(1)
	return Config{Name: "上游 A", Mode: "sub2api", Currency: "USD", AccountIDs: []int64{1, 2}, QueryAccountID: &id, AutoEnabled: true, IntervalMinutes: 30}
}
func exampleAccount() *service.Account {
	return &service.Account{ID: 1, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fake-test-key", "base_url": "https://relay.example/v1"}}
}
func TestConfigAndExactAmounts(t *testing.T) {
	for _, v := range []string{"0", "-0.000001", "999999999.123456"} {
		n, e := amount(v, true)
		require.NoError(t, e)
		require.Equal(t, numeric(v), numeric(n))
	}
	for _, v := range []string{"NaN", "1e10", "1000000000", "0.0000001", " 1", "1;DROP TABLE accounts"} {
		_, e := amount(v, true)
		require.Error(t, e)
	}
	c := exampleConfig()
	require.NoError(t, c.validate())
	c.AccountIDs = []int64{1, 1}
	require.Error(t, c.validate())
	c = exampleConfig()
	c.AccountIDs = []int64{2}
	require.Error(t, c.validate())
	c = exampleConfig()
	c.Mode = "manual"
	c.EstimateEnabled = true
	c.Currency = "CNY"
	require.Error(t, c.validate())
	c = exampleConfig()
	c.ManagementURL = "https://password@example.com"
	require.Error(t, c.validate())
	c = exampleConfig()
	c.Mode = "manual"
	require.NoError(t, c.validate())
	require.Nil(t, c.QueryAccountID)
	require.False(t, c.AutoEnabled)
}
func TestResponseScopesNeverInventWalletFunds(t *testing.T) {
	now := time.Now()
	cases := []struct{ body, scope, value string }{
		{`{"mode":"unrestricted","unit":"USD","balance":100,"remaining":100}`, "wallet", "100.000000"},
		{`{"mode":"unrestricted","unit":"USD","balance":"-0.000001"}`, "wallet", "-0.000001"},
		{`{"mode":"unrestricted","unit":"USD","balance":99.9999999999}`, "wallet", "100.000000"},
		{`{"mode":"unrestricted","unit":"USD","balance":1e-6}`, "wallet", "0.000001"},
		{`{"mode":"quota_limited","unit":"USD","remaining":100,"quota":{"remaining":20,"unit":"USD"}}`, "key_quota", "20.000000"},
		{`{"mode":"unrestricted","unit":"USD","remaining":30,"subscription":{"daily_limit_usd":50}}`, "subscription", "30.000000"},
	}
	for _, c := range cases {
		o, e := parseObservation([]byte(c.body), now)
		require.NoError(t, e)
		require.Equal(t, c.scope, o.Scope)
		require.Equal(t, c.value, o.Amount)
		require.Equal(t, now, o.At)
	}
	for _, body := range []string{`{}`, `{"mode":"unrestricted","unit":"USD","remaining":100}`, `{"mode":"quota_limited","unit":"USD","remaining":100}`, `{"mode":"unrestricted","balance":100}`, `{"mode":"unrestricted","unit":"USD","balance":null}`, `{"mode":"unrestricted","unit":"USD","remaining":50,"subscription":{}}`, `{"data":{"balance":100}}`} {
		_, e := parseObservation([]byte(body), now)
		require.Error(t, e, body)
	}
}
func TestForecastAndEstimateDoNotDoubleSubtractActualFunds(t *testing.T) {
	now := time.Now()
	w := Wallet{Config: exampleConfig(), Status: "ok", Observation: &Observation{Amount: "100.000000", Currency: "USD", Scope: "wallet", At: now}, WeekCost: "70", SinceBaselineCost: "25"}
	w.decorate(now)
	require.Nil(t, w.EstimatedBalance)
	require.Equal(t, "10.0", *w.DaysRemaining)
	require.Equal(t, "100.000000", w.Observation.Amount)
	w.Mode = "manual"
	w.EstimateEnabled = true
	w.Observation.Scope = "manual"
	w.decorate(now)
	require.Equal(t, "75.000000", *w.EstimatedBalance)
	require.Equal(t, "7.5", *w.DaysRemaining)
	w = Wallet{Config: exampleConfig(), Status: "failed", Observation: &Observation{Amount: "100", Currency: "USD", Scope: "wallet", At: now}, WeekCost: "70"}
	w.decorate(now)
	require.True(t, w.Stale)
	require.Nil(t, w.DaysRemaining)
	w = Wallet{Config: exampleConfig(), Status: "ok", Observation: &Observation{Amount: "100", Currency: "USD", Scope: "key_quota", At: now}, WeekCost: "70"}
	w.decorate(now)
	require.Nil(t, w.DaysRemaining)
}
func TestProbeReadOnlyTargetAndSanitizedFailure(t *testing.T) {
	a := exampleAccount()
	p := &Probe{Config: &config.Config{}, Send: func(req *http.Request, proxy string) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "https://relay.example/v1/usage", req.URL.String())
		require.Equal(t, "Bearer fake-test-key", req.Header.Get("Authorization"))
		require.Empty(t, proxy)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"mode":"unrestricted","unit":"USD","balance":100}`))}, nil
	}}
	before, _ := json.Marshal(a)
	o, e := p.Query(context.Background(), a)
	require.NoError(t, e)
	require.Equal(t, "wallet", o.Scope)
	after, _ := json.Marshal(a)
	require.Equal(t, before, after)
	p.Send = func(*http.Request, string) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("private body fake-test-key"))}, nil
	}
	_, e = p.Query(context.Background(), a)
	require.EqualError(t, e, "authentication_failed")
	for _, raw := range []string{"http://relay.example", "https://localhost", "https://127.0.0.1", "https://169.254.169.254", "https://user:password@relay.example", "https://relay.example?key=secret"} {
		_, e = probeURL(raw, &config.Config{})
		require.Error(t, e, raw)
	}
	for _, raw := range []string{"https://relay.example", "https://relay.example/v1/", "https://relay.example/prefix/v1"} {
		target, e := probeURL(raw, &config.Config{})
		require.NoError(t, e)
		require.True(t, strings.HasSuffix(target, "/v1/usage"))
		require.NotContains(t, target, "/v1/v1")
	}
}
func TestSourceFingerprintAndSettings(t *testing.T) {
	a := exampleAccount()
	first := sourceIdentity(a)
	a.Credentials["api_key"] = "another-key"
	require.NotEqual(t, first, sourceIdentity(a))
	require.NotContains(t, sourceIdentity(a), "another-key")
	s := Settings{}
	require.NoError(t, s.validate())
	require.NotNil(t, s.Recipients)
	s.EmailEnabled = true
	require.Error(t, s.validate())
	s.Recipients = []string{"test@example.com"}
	require.NoError(t, s.validate())
}
