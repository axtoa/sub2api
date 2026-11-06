package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// Poll persisted studio tasks through the same credential, URL and proxy
// implementation as the Grok gateway, without depending on a browser request.
func (s *OpenAIGatewayService) GetCreativeGrokVideoStatus(ctx context.Context, account *Account, requestID string) (*CreativeVideoProviderStatus, error) {
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	target, err := buildGrokMediaURL(account, s.cfg, GrokMediaEndpointVideoStatus, requestID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if account.IsGrokOAuth() && isGrokCLIProxyTarget(target) {
		applyGrokCLIHeaders(req.Header)
	}
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil {
		if account.Proxy == nil {
			return nil, fmt.Errorf("media account proxy is unavailable")
		}
		proxy = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("video status returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("invalid video status response")
	}
	status := creativeVideoStatusFromGrokStatus(gjson.GetBytes(body, "status").String(), IsGrokVideoStatusBillable(body))
	// A done response without a result URL must not be settled yet.
	if status == CreativeVideoStatusCompleted && !IsGrokVideoStatusBillable(body) {
		status = CreativeVideoStatusRunning
	}
	return &CreativeVideoProviderStatus{
		ID: requestID, Status: status,
		Model:           strings.TrimSpace(gjson.GetBytes(body, "model").String()),
		DownloadURL:     strings.TrimSpace(gjson.GetBytes(body, "video.url").String()),
		DurationSeconds: int(gjson.GetBytes(body, "video.duration").Float()),
	}, nil
}
