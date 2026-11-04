package upstreambalance

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewAPIWalletAndKeyQuotaAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		body, scope, amount string
		ratio               int64
	}{
		{`{"code":true,"data":{"total_available":500000,"user_quota":{"total_available":52500000}}}`, "wallet", "105.000000", 500000},
		{`{"code":true,"data":{"total_available":500000,"unlimited_quota":false}}`, "key_quota", "1.000000", 500000},
		{`{"code":true,"data":{"user_quota":{"total_available":1000000000000}}}`, "wallet", "1.000000", 1000000000000},
		{`{"success":true,"data":{"total_available":0,"unlimited_quota":true,"user_quota":{"total_available":0}}}`, "wallet", "0.000000", 0},
		{`{"code":true,"data":{"total_available":1234567890,"user_quota":{"total_available":987654321}}}`, "wallet", "987.654321", 1000000},
	} {
		o, err := parseNewAPIObservation([]byte(tc.body), tc.ratio, time.Now())
		require.NoError(t, err)
		require.Equal(t, tc.scope, o.Scope)
		require.Equal(t, tc.amount, o.Amount)
		require.Equal(t, "USD", o.Currency)
	}
	for _, body := range []string{`{}`, `{"code":false,"data":{"total_available":5}}`, `{"success":true,"data":{}}`, `{"code":true,"data":{"total_available":0,"unlimited_quota":true}}`, `{"code":true,"data":{"total_available":5,"user_quota":{}}}`} {
		_, err := parseNewAPIObservation([]byte(body), 500000, time.Now())
		require.Error(t, err, body)
	}
}

func TestNewAPIProbeUsesReadOnlyTokenEndpoint(t *testing.T) {
	a := exampleAccount()
	p := &Probe{Config: &config.Config{}, Send: func(req *http.Request, proxy string) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "https://relay.example/api/usage/token/", req.URL.String())
		require.Equal(t, "Bearer fake-test-key", req.Header.Get("Authorization"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":true,"data":{"unlimited_quota":true,"user_quota":{"total_available":1000000}}}`))}, nil
	}}
	c := exampleConfig()
	c.Mode = "newapi"
	require.NoError(t, c.validate())
	require.EqualValues(t, 500000, c.QuotaPerUnit)
	o, err := p.QueryWallet(context.Background(), a, c)
	require.NoError(t, err)
	require.Equal(t, "2.000000", o.Amount)
	w := Wallet{Config: c, Observation: o, Status: "failed"}
	w.decorate(time.Now())
	require.True(t, w.Stale)
	c.QuotaPerUnit = -1
	require.Error(t, c.validate())
	c.QuotaPerUnit = 500000
	c.Currency = "CNY"
	require.Error(t, c.validate())
}
