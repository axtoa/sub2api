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

func TestCreativeVideoHTTPProvider_MiniMaxV2SubmitAndStatus(t *testing.T) {
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v2/video_generation":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), `"model":"MiniMax-H3"`)
			require.Contains(t, string(body), `"resolution":"768P"`)
			require.Contains(t, string(body), `"ratio":"9:16"`)
			require.Contains(t, string(body), `"content"`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"task_id":"task_h3","status":"Queueing","model":"MiniMax-H3"}`)),
			}, nil
		case "/v2/query/video_generation/task_h3":
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"task":{"task_id":"task_h3","status":"Success","model":"MiniMax-H3","resolution":"768P","duration":6,"content":{"url":"https://minimax.test/video-h3.mp4"}}}`)),
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

	submitted, err := provider.Submit(context.Background(), account, CreativeVideoProviderRequest{
		Model:       "minimax-h3",
		Prompt:      "rainy street",
		AspectRatio: "9:16",
		Resolution:  "720p",
		Duration:    6,
	})
	require.NoError(t, err)
	require.Equal(t, "task_h3", submitted.ID)
	require.Equal(t, CreativeVideoStatusRunning, submitted.Status)

	status, err := provider.Get(context.Background(), account, "task_h3")
	require.NoError(t, err)
	require.Equal(t, CreativeVideoStatusCompleted, status.Status)
	require.Equal(t, "https://minimax.test/video-h3.mp4", status.DownloadURL)
	require.Equal(t, VideoBillingResolution720P, status.Resolution)
	require.Equal(t, 6, status.DurationSeconds)
}

func TestCreativeVideoHTTPProvider_MiniMaxHappyCodeRelay(t *testing.T) {
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/videos/generations":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), `"model":"MiniMax-H3"`)
			require.Contains(t, string(body), `"resolution":"768P"`)
			require.Contains(t, string(body), `"ratio":"9:16"`)
			require.Contains(t, string(body), `"content"`)
			require.NotContains(t, string(body), `"prompt"`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"request_id":"vid_happycode","model":"MiniMax-H3","status":"pending","created_at":1789099200}`)),
			}, nil
		case "/v1/videos/vid_happycode":
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"request_id":"vid_happycode","model":"MiniMax-H3","status":"done","video":{"url":"/v1/videos/vid_happycode/content","duration":8,"resolution":"768p"}}`)),
			}, nil
		case "/v1/videos/vid_happycode/content":
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"video/mp4"}},
				Body:       io.NopCloser(strings.NewReader("happycode-mp4")),
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
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://happycodeai.com/v1"},
	}

	submitted, err := provider.Submit(context.Background(), account, CreativeVideoProviderRequest{
		Model:       "MiniMax-H3",
		Prompt:      "sunrise ocean",
		AspectRatio: "9:16",
		Resolution:  "720p",
		Duration:    8,
	})
	require.NoError(t, err)
	require.Equal(t, "vid_happycode", submitted.ID)
	require.Equal(t, CreativeVideoStatusRunning, submitted.Status)

	status, err := provider.Get(context.Background(), account, "vid_happycode")
	require.NoError(t, err)
	require.Equal(t, CreativeVideoStatusCompleted, status.Status)
	require.Equal(t, "/v1/videos/vid_happycode/content", status.DownloadURL)
	require.Equal(t, VideoBillingResolution720P, status.Resolution)
	require.Equal(t, 8, status.DurationSeconds)

	body, contentType, err := provider.OpenContent(context.Background(), account, status)
	require.NoError(t, err)
	defer func() { _ = body.Close() }()
	require.Equal(t, "video/mp4", contentType)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "happycode-mp4", string(data))
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
