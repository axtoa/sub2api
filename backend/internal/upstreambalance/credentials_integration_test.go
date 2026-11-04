//go:build integration

package upstreambalance

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostgresAccountCredentialLifecycleAndNoSecretCopies(t *testing.T) {
	e := engine(t)
	e.Store.Cipher = credentialCipher(t)
	ctx := context.Background()
	token := "account-credential-canary"
	in := input()
	in.Mode, in.NewAPIAuth, in.NewAPIUserID, in.AccessToken = "newapi", "account", 17, &token
	w, err := e.Store.Save(ctx, "", in, 42)
	require.NoError(t, err)
	require.True(t, w.HasAccessToken)
	replayed, err := e.Store.Save(ctx, "", in, 42)
	require.NoError(t, err)
	require.Equal(t, w.ID, replayed.ID)
	require.True(t, replayed.HasAccessToken)
	var encrypted, storedConfig, responses, events string
	require.NoError(t, e.Store.DB.QueryRow(`SELECT access_token_encrypted,config::text FROM upstream_wallets WHERE id=$1`, w.ID).Scan(&encrypted, &storedConfig))
	require.NoError(t, e.Store.DB.QueryRow(`SELECT response::text FROM upstream_wallet_operations WHERE id=$1`, in.RequestKey).Scan(&responses))
	require.NoError(t, e.Store.DB.QueryRow(`SELECT detail::text FROM upstream_wallet_events WHERE wallet_id=$1`, w.ID).Scan(&events))
	require.NotEmpty(t, encrypted)
	for _, raw := range []string{encrypted, storedConfig, responses, events} {
		require.NotContains(t, raw, token)
	}
	for _, raw := range []string{storedConfig, responses, events} {
		require.NotContains(t, raw, encrypted)
	}
	visible, err := json.Marshal(w)
	require.NoError(t, err)
	require.NotContains(t, string(visible), token)
	require.NotContains(t, string(visible), encrypted)
	calls := 0
	e.Probe.Send = func(req *http.Request, _ string) (*http.Response, error) {
		calls++
		require.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
		require.Equal(t, "/api/user/self", req.URL.Path)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"data":{"id":17,"quota":52500000}}`))}, nil
	}
	w, err = e.Query(ctx, w.ID, true)
	require.NoError(t, err)
	require.Equal(t, "ok", w.Status)
	require.Equal(t, "105.000000", w.Observation.Amount)
	edit := Input{Config: w.Config, Version: w.Version, RequestKey: uuid.NewString()}
	w, err = e.Store.Save(ctx, w.ID, edit, 42)
	require.NoError(t, err)
	require.Equal(t, encrypted, w.accessTokenEncrypted) // blank keeps the credential
	require.NotNil(t, w.Observation)
	source := e.Accounts.(*accountSource)
	source.changeKey() // account tokens survive model-key rotation
	_, err = e.Query(ctx, w.ID, true)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	source.mu.Lock()
	source.a.Credentials["base_url"] = "https://changed.example/v1"
	source.mu.Unlock()
	w, err = e.Query(ctx, w.ID, true)
	require.NoError(t, err)
	require.Equal(t, "credential_source_changed", w.LastError)
	require.Equal(t, 2, calls) // no credential sent to a changed destination
	edit.Version, edit.RequestKey = w.Version, uuid.NewString()
	_, err = e.Store.Save(ctx, w.ID, edit, 42)
	require.Error(t, err) // keeping a credential cannot silently rebind it
	empty := ""
	edit.NewAPIAuth, edit.AccessToken, edit.RequestKey = "key", &empty, uuid.NewString()
	w, err = e.Store.Save(ctx, w.ID, edit, 42)
	require.NoError(t, err)
	require.False(t, w.HasAccessToken)
	require.Empty(t, w.accessTokenEncrypted)
	require.Nil(t, w.Observation)
	// A new token can explicitly bind to the changed source.
	edit = Input{Config: w.Config, Version: w.Version, RequestKey: uuid.NewString(), AccessToken: &token}
	edit.NewAPIAuth = "account"
	w, err = e.Store.Save(ctx, w.ID, edit, 42)
	require.NoError(t, err)
	require.True(t, w.HasAccessToken)
	edit = Input{Config: w.Config, Version: w.Version, RequestKey: uuid.NewString()}
	edit.Mode = "sub2api"
	w, err = e.Store.Save(ctx, w.ID, edit, 42)
	require.NoError(t, err)
	require.False(t, w.HasAccessToken)
	require.Empty(t, w.accessTokenEncrypted)
	// Historical operations/audits must not retain the old credential either.
	rows, err := e.Store.DB.Query(`SELECT detail::text FROM upstream_wallet_events WHERE wallet_id=$1 UNION ALL SELECT response::text FROM upstream_wallet_operations`, w.ID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var raw string
		require.NoError(t, rows.Scan(&raw))
		require.NotContains(t, raw, token)
		require.NotContains(t, raw, encrypted)
	}
	require.NoError(t, rows.Err())
}

func TestPostgresUnconfiguredEncryptionNeverStoresCredential(t *testing.T) {
	e := engine(t)
	token := "cannot-store-with-an-ephemeral-key"
	in := input()
	in.Mode, in.NewAPIAuth, in.AccessToken = "newapi", "account", &token
	_, err := e.Store.Save(context.Background(), "", in, 42)
	require.Error(t, err)
	var count int
	require.NoError(t, e.Store.DB.QueryRow(`SELECT COUNT(*) FROM upstream_wallets`).Scan(&count))
	require.Zero(t, count)
}
