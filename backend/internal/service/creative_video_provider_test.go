package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreativeVideoHTTPProvider_OpenAISubmit(t *testing.T) {
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/v1/videos", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"vid_123","status":"queued","model":"sora-2"}`)),
		}, nil
	})}
	provider := NewCreativeVideoHTTPProvider(PlatformOpenAI, client)
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://openai.test"},
	}

	got, err := provider.Submit(context.Background(), account, CreativeVideoProviderRequest{
		Model:      "sora-2",
		Prompt:     "a calm ocean",
		Resolution: "720p",
		Duration:   5,
	})

	require.NoError(t, err)
	require.Equal(t, "vid_123", got.ID)
	require.Equal(t, CreativeVideoStatusRunning, got.Status)
	require.Equal(t, "sora-2", got.Model)
}

func TestCreativeVideoHTTPProvider_MiniMaxStatusAndDownload(t *testing.T) {
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/query/video_generation":
			require.Equal(t, "task_123", r.URL.Query().Get("task_id"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"task_id":"task_123","status":"Success","file_id":"file_123"}`)),
			}, nil
		case "/v1/files/retrieve":
			require.Equal(t, "file_123", r.URL.Query().Get("file_id"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"file":{"download_url":"https://minimax.test/video.mp4"}}`)),
			}, nil
		case "/video.mp4":
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"video/mp4"}},
				Body:       io.NopCloser(strings.NewReader("mp4")),
			}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
	})}
	provider := NewCreativeVideoHTTPProvider(PlatformMiniMax, client)
	account := &Account{
		ID:          1,
		Platform:    PlatformMiniMax,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://minimax.test/v1"},
	}

	status, err := provider.Get(context.Background(), account, "task_123")
	require.NoError(t, err)
	require.Equal(t, CreativeVideoStatusCompleted, status.Status)
	require.Equal(t, "file_123", status.FileID)

	body, contentType, err := provider.OpenContent(context.Background(), account, status)
	require.NoError(t, err)
	defer func() { _ = body.Close() }()
	require.Equal(t, "video/mp4", contentType)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "mp4", string(data))
}
