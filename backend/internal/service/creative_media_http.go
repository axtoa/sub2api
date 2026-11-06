package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

// Media uses the account's egress settings without forwarding private headers
// to a provider's external artifact CDN.
func creativeMediaAccountClient(account *Account, client *http.Client) (*http.Client, error) {
	useDefaultClient := client == nil || client == batchImageDefaultHTTPClient()
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	if account == nil {
		return client, nil
	}
	if account.ProxyID != nil && *account.ProxyID > 0 && account.Proxy == nil {
		return nil, fmt.Errorf("media account proxy is unavailable")
	}
	if account.ProxyID != nil && account.Proxy != nil {
		var err error
		client, err = httpclient.GetClient(httpclient.Options{ProxyURL: account.Proxy.URL(), ResponseHeaderTimeout: 60 * time.Second})
		if err != nil {
			return nil, err
		}
	}
	var external http.RoundTripper
	if useDefaultClient {
		if account.ProxyID != nil && account.Proxy != nil {
			secureClient, err := httpclient.GetClient(httpclient.Options{ProxyURL: account.Proxy.URL(), ResponseHeaderTimeout: 60 * time.Second, ValidateResolvedIP: true})
			if err != nil {
				return nil, err
			}
			external = secureClient.Transport
		} else if transport, ok := http.DefaultTransport.(*http.Transport); ok {
			secureTransport := transport.Clone()
			secureTransport.Proxy = nil
			secureTransport.DialContext = safeDialContext
			external = secureTransport
		}
	}
	copyClient := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	origin, _ := url.Parse(account.GetOpenAIBaseURL())
	copyClient.Transport = creativeMediaTransport{base: transport, external: external, account: account, origin: origin}
	previousRedirect := client.CheckRedirect
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many media redirects")
		}
		if previousRedirect != nil {
			if err := previousRedirect(req, via); err != nil {
				return err
			}
		}
		if len(via) > 0 {
			if err := validateCreativeMediaTarget(req.URL, origin); err != nil {
				return err
			}
		}
		if len(via) > 0 && !sameCreativeMediaOrigin(req.URL, via[0].URL) {
			// net/http otherwise preserves credentials on subdomain redirects.
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			req.Header.Del("Proxy-Authorization")
		}
		return nil
	}
	return &copyClient, nil
}

func sameCreativeMediaOrigin(a, b *url.URL) bool {
	return a != nil && b != nil && strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) && effectivePort(a) == effectivePort(b)
}

type creativeMediaTransport struct {
	base     http.RoundTripper
	external http.RoundTripper
	account  *Account
	origin   *url.URL
}

func (t creativeMediaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateCreativeMediaTarget(req.URL, t.origin); err != nil {
		return nil, err
	}
	if sameCreativeMediaOrigin(req.URL, t.origin) {
		req = req.Clone(req.Context())
		t.account.ApplyHeaderOverrides(req.Header)
		return t.base.RoundTrip(req)
	}
	if t.external != nil {
		return t.external.RoundTrip(req)
	}
	if transport, ok := t.base.(*http.Transport); ok {
		copyTransport := transport.Clone()
		copyTransport.DisableKeepAlives = true
		var proxyURL *url.URL
		if copyTransport.Proxy != nil {
			var err error
			proxyURL, err = copyTransport.Proxy(req)
			if err != nil {
				return nil, err
			}
		}
		if proxyURL == nil {
			copyTransport.DialContext = safeDialContext
		} else {
			blocked, err := isPrivateOrLoopbackHost(req.Context(), req.URL.Hostname())
			if err != nil {
				return nil, err
			}
			if blocked {
				return nil, fmt.Errorf("unsafe external media URL")
			}
		}
		return copyTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

func validateCreativeMediaTarget(target, origin *url.URL) error {
	if target == nil || target.Hostname() == "" || target.User != nil {
		return fmt.Errorf("invalid media URL")
	}
	if sameCreativeMediaOrigin(target, origin) {
		return nil
	}
	if !strings.EqualFold(target.Scheme, "https") || urlvalidator.IsBlockedHost(target.Hostname()) || isBlockedHostname(target.Hostname()) {
		return fmt.Errorf("unsafe external media URL")
	}
	return nil
}
