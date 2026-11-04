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
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func credentialCipher(t *testing.T) service.SecretEncryptor {
	t.Helper()
	cipher, err := repository.NewAESEncryptor(&config.Config{Totp: config.TotpConfig{EncryptionKey: strings.Repeat("a1", 32), EncryptionKeyConfigured: true}})
	require.NoError(t, err)
	return cipher
}

func TestWalletCredentialValidationAndFixedEncryptionKey(t *testing.T) {
	for _, token := range []string{"abc\ndef", "Bearer abc", strings.Repeat("x", 4097)} {
		in := Input{Config: exampleConfig(), AccessToken: &token}
		in.Mode, in.NewAPIAuth = "newapi", "account"
		require.Error(t, in.validate())
	}
	token := "private-account-token"
	in := Input{Config: exampleConfig(), AccessToken: &token}
	require.Error(t, in.validate()) // Sub2API does not accept an account token.
	in.Mode, in.NewAPIAuth = "newapi", "account"
	require.NoError(t, in.validate())
	in.NewAPIAuth = "unknown"
	require.Error(t, in.validate())
	in.NewAPIAuth, in.NewAPIUserID = "account", -1
	require.Error(t, in.validate())
	for _, cfg := range []*config.Config{nil, {}, {Totp: config.TotpConfig{EncryptionKey: strings.Repeat("aa", 32)}}} {
		require.Nil(t, NewEngine(nil, nil, nil, cfg).Store.Cipher)
	}
	e := NewEngine(nil, nil, nil, &config.Config{Totp: config.TotpConfig{EncryptionKey: strings.Repeat("aa", 32), EncryptionKeyConfigured: true}})
	require.NotNil(t, e.Store.Cipher)
}

func TestWalletCredentialEncryptionBindingAndNoReadback(t *testing.T) {
	cipher := credentialCipher(t)
	a := exampleAccount()
	w := Wallet{ID: "wallet-1", Config: exampleConfig(), HasAccessToken: true}
	raw, err := json.Marshal(walletCredential{WalletID: w.ID, Binding: credentialBinding(a), Token: "private-account-token"})
	require.NoError(t, err)
	w.accessTokenEncrypted, err = cipher.Encrypt(string(raw))
	require.NoError(t, err)
	token, err := accountCredential(cipher, w, a)
	require.NoError(t, err)
	require.Equal(t, "private-account-token", token)
	visible, err := json.Marshal(w)
	require.NoError(t, err)
	require.NotContains(t, string(visible), token)
	require.NotContains(t, string(visible), w.accessTokenEncrypted)
	a.Credentials["api_key"] = "rotated-model-key"
	_, err = accountCredential(cipher, w, a)
	require.NoError(t, err)
	a.Credentials["base_url"] = "https://other.example/v1"
	_, err = accountCredential(cipher, w, a)
	require.EqualError(t, err, "credential_source_changed")
	w.ID = "wallet-2"
	_, err = accountCredential(cipher, w, exampleAccount())
	require.EqualError(t, err, "credential_decryption_failed")
	w.ID, w.accessTokenEncrypted = "wallet-1", "tampered"
	_, err = accountCredential(cipher, w, exampleAccount())
	require.EqualError(t, err, "credential_decryption_failed")
}

func TestNewAPIAccountQueryUsesOnlyBoundReadOnlyEndpoint(t *testing.T) {
	a := exampleAccount()
	c := exampleConfig()
	c.Mode, c.NewAPIAuth, c.NewAPIUserID = "newapi", "account", 17
	calls := 0
	p := &Probe{Send: func(req *http.Request, _ string) (*http.Response, error) {
		calls++
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "https://relay.example/api/user/self", req.URL.String())
		require.Equal(t, "Bearer private-account-token", req.Header.Get("Authorization"))
		require.Equal(t, "17", req.Header.Get("New-Api-User"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"id":17,"quota":52500000,"used_quota":999999}}`))}, nil
	}}
	_, err := p.QueryWallet(context.Background(), a, c)
	require.EqualError(t, err, "credential_missing")
	require.Zero(t, calls)
	o, err := p.queryWallet(context.Background(), a, c, "private-account-token")
	require.NoError(t, err)
	require.Equal(t, "105.000000", o.Amount)
	require.Equal(t, "wallet", o.Scope)
	require.Equal(t, 1, calls)
	for _, body := range []string{`{}`, `{"success":true,"data":{"id":17}}`, `{"success":false,"data":{"id":17,"quota":5}}`, `{"success":true,"data":{"id":17,"quota":"invalid"}}`} {
		_, err = parseNewAPIAccountObservation([]byte(body), 500000, 17, time.Now())
		require.Error(t, err)
	}
	_, err = parseNewAPIAccountObservation([]byte(`{"success":true,"data":{"id":18,"quota":5}}`), 500000, 17, time.Now())
	require.EqualError(t, err, "credential_user_mismatch")
	_, err = parseNewAPIAccountObservation([]byte(`{"success":false,"message":"private-upstream-credential"}`), 500000, 17, time.Now())
	require.EqualError(t, err, "authentication_failed")
	o, err = parseNewAPIAccountObservation([]byte(`{"success":true,"data":{"id":17,"quota":0}}`), 500000, 0, time.Now())
	require.NoError(t, err)
	require.Equal(t, "0.000000", o.Amount)
}
