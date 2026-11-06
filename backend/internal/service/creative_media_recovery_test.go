//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreativeImageDeferredResumeDoesNotRegenerateCompletedItems(t *testing.T) {
	root := t.TempDir()
	calls := 0
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "16:9", payload["aspect_ratio"])
		require.NotContains(t, payload, "size")
		refs := payload["subject_reference"].([]any)
		require.Equal(t, "character", refs[0].(map[string]any)["type"])
		require.Contains(t, refs[0].(map[string]any)["image_file"], "data:image/png;base64,")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"image_urls":["data:image/png;base64,iVBORw0KGgo="]},"base_resp":{"status_code":0}}`)), Header: http.Header{}}, nil
	})}
	provider := NewMiniMaxBatchImageProvider(MiniMaxBatchImageProviderOptions{ResultDir: root, Deferred: true}, client)
	account := &Account{Platform: PlatformMiniMax, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "https://minimax.test"}}
	item := BatchImageInputItem{CustomID: "one", Prompt: "draw", ReferenceImages: []BatchImageReference{{MimeType: "image/png", Data: batchImageProviderTinyPNG}}}
	other := item
	other.CustomID = "two"
	input := BatchImageInput{BatchID: "imgbatch_resume", Model: "image-01", AspectRatio: "16:9", Items: []BatchImageInputItem{item, other}}
	created, err := provider.Submit(context.Background(), &BatchImageJob{BatchID: input.BatchID}, account, input)
	require.NoError(t, err)
	inputPath, err := batchImageLocalResultPath(root, created.ProviderInputRef)
	require.NoError(t, err)
	inputInfo, err := os.Stat(inputPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), inputInfo.Mode().Perm())
	require.Zero(t, calls, "HTTP submission must only persist work")
	job := &BatchImageJob{BatchID: input.BatchID, ProviderInputRef: &created.ProviderInputRef}
	status, err := provider.Get(context.Background(), job, account)
	require.NoError(t, err)
	require.False(t, status.Done)
	require.Equal(t, 1, calls)
	// Recreate the provider as after a process restart.
	provider = NewMiniMaxBatchImageProvider(MiniMaxBatchImageProviderOptions{ResultDir: root, Deferred: true}, client)
	status, err = provider.Get(context.Background(), job, account)
	require.NoError(t, err)
	require.True(t, status.Done)
	require.Equal(t, 2, calls)
	_, err = provider.Get(context.Background(), job, account)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

func TestCreativeImageUncertainSubmissionIsNotRepeated(t *testing.T) {
	root := t.TempDir()
	input := BatchImageInput{BatchID: "imgbatch_uncertain", Model: "image-01", Items: []BatchImageInputItem{{CustomID: "one", Prompt: "draw"}}}
	created, err := submitCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, input)
	require.NoError(t, err)
	path, err := batchImageLocalResultPath(root, created.ProviderInputRef)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(path+".parts", 0700))
	require.NoError(t, os.WriteFile(filepath.Join(path+".parts", "000000.jsonl.started"), nil, 0600))
	stale := time.Now().Add(-10 * time.Minute)
	require.NoError(t, os.Chtimes(filepath.Join(path+".parts", "000000.jsonl.started"), stale, stale))
	job := &BatchImageJob{BatchID: input.BatchID, ProviderInputRef: &created.ProviderInputRef}
	status, err := processCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, job, &Account{}, nil, func(context.Context, *batchImageHTTPClient, BatchImageInput, BatchImageInputItem) ([]providerImageResult, error) {
		t.Fatal("must not resend uncertain upstream request")
		return nil, nil
	})
	require.NoError(t, err)
	require.True(t, status.Done)
	output, _ := batchImageLocalResultPath(root, status.ProviderOutputRef)
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Contains(t, string(data), "UPSTREAM_RESULT_UNCERTAIN")
}

func TestCreativeMediaHeadersStayOnUpstreamOrigin(t *testing.T) {
	account := &Account{Platform: PlatformMiniMax, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://upstream.test", "api_key": "test", credKeyHeaderOverrideEnabled: true,
		credKeyHeaderOverrides: map[string]any{"x-custom": "private-value"},
	}}
	calls := 0
	client, err := creativeMediaAccountClient(account, &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host == "upstream.test" {
			require.Equal(t, []string{"private-value"}, r.Header["x-custom"])
		} else {
			require.Empty(t, r.Header["x-custom"])
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})})
	require.NoError(t, err)
	for _, target := range []string{"https://upstream.test/v1/videos", "https://cdn.test/output"} {
		response, err := client.Get(target)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
	}
	require.Equal(t, 2, calls)
}

func TestCreativeImageCancelPreservesCompletedOutput(t *testing.T) {
	root := t.TempDir()
	input := BatchImageInput{BatchID: "imgbatch_cancel", Model: "image-01", Items: []BatchImageInputItem{{CustomID: "one"}, {CustomID: "two"}}}
	created, err := submitCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, input)
	require.NoError(t, err)
	job := &BatchImageJob{BatchID: input.BatchID, ProviderInputRef: &created.ProviderInputRef}
	calls := 0
	generate := func(context.Context, *batchImageHTTPClient, BatchImageInput, BatchImageInputItem) ([]providerImageResult, error) {
		calls++
		return []providerImageResult{{MimeType: "image/png", Data: []byte("image")}}, nil
	}
	status, err := processCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, job, &Account{}, nil, generate)
	require.NoError(t, err)
	require.False(t, status.Done)
	require.NoError(t, cancelCreativeImageInput(root, job))
	status, err = processCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, job, &Account{}, nil, generate)
	require.NoError(t, err)
	require.True(t, status.Done)
	require.Equal(t, 1, calls)
	path, err := batchImageLocalResultPath(root, status.ProviderOutputRef)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "CANCELLED")
	require.Contains(t, string(data), "aW1hZ2U=")
}

func TestCreativeImagePriorityAndModelMapping(t *testing.T) {
	svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
	accounts := svc.AccountRepo.(*publicBatchImageAccountRepo)
	low := testBatchImageAccount(101, AccountTypeAPIKey)
	high := testBatchImageAccount(102, AccountTypeAPIKey)
	low.Priority = 20
	high.Priority = 1
	req := validBatchImageSubmitRequest()
	high.Credentials["model_mapping"] = map[string]any{req.Model: "image-upstream-alias"}
	accounts.accounts = []Account{low, high}
	got, err := svc.Submit(context.Background(), testBatchImageOwner(), req, "")
	require.NoError(t, err)
	require.Equal(t, int64(102), *repo.jobs[got.ID].AccountID)
	require.Equal(t, req.Model, repo.jobs[got.ID].Model)
	require.Equal(t, "image-upstream-alias", provider.submits[0].Model)
}

func TestCreativeImageConcurrentWorkerDoesNotReplaceActiveResult(t *testing.T) {
	root := t.TempDir()
	input := BatchImageInput{BatchID: "imgbatch_concurrent", Model: "image-01", Items: []BatchImageInputItem{{CustomID: "one", Prompt: "draw"}}}
	created, err := submitCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, input)
	require.NoError(t, err)
	job := &BatchImageJob{BatchID: input.BatchID, ProviderInputRef: &created.ProviderInputRef}
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := processCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, job, &Account{}, nil, func(context.Context, *batchImageHTTPClient, BatchImageInput, BatchImageInputItem) ([]providerImageResult, error) {
			close(started)
			<-release
			return []providerImageResult{{MimeType: "image/png", Data: []byte("completed image")}}, nil
		})
		done <- err
	}()
	defer func() { close(release); require.NoError(t, <-done) }()
	<-started
	status, err := processCreativeImageInput(context.Background(), root, BatchImageProviderMiniMax, job, &Account{}, nil, func(context.Context, *batchImageHTTPClient, BatchImageInput, BatchImageInputItem) ([]providerImageResult, error) {
		t.Error("second worker must not generate")
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, BatchProviderStateRunning, status.InternalState)
	require.False(t, status.Done)
	path, err := batchImageLocalResultPath(root, created.ProviderInputRef)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(path+".parts", "000000.jsonl"))
	require.True(t, os.IsNotExist(err), "second worker must not persist a false failure")
}

func TestCreativeMediaRedirectStripsCredentialsOnSubdomain(t *testing.T) {
	account := &Account{Platform: PlatformMiniMax, Credentials: map[string]any{"base_url": "https://upstream.test"}}
	calls := 0
	client, err := creativeMediaAccountClient(account, &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://cdn.upstream.test/output"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("image")), Request: r}, nil
	})})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodGet, "https://upstream.test/content", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "session=secret")
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, 2, calls)
}

func TestCreativeMediaRejectsPrivateAndInsecureExternalTargets(t *testing.T) {
	origin, err := url.Parse("https://provider.example")
	require.NoError(t, err)
	for _, raw := range []string{"http://cdn.example/image", "https://127.0.0.1/image", "https://169.254.169.254/latest/meta-data", "https://user:pass@cdn.example/image"} {
		target, parseErr := url.Parse(raw)
		require.NoError(t, parseErr)
		require.Error(t, validateCreativeMediaTarget(target, origin), raw)
	}
	target, err := url.Parse("https://cdn.example/image")
	require.NoError(t, err)
	require.NoError(t, validateCreativeMediaTarget(target, origin))
}
