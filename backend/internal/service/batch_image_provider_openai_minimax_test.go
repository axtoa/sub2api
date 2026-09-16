package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type batchImageProviderRoundTripFunc func(*http.Request) (*http.Response, error)

func (f batchImageProviderRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

var batchImageProviderTinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89,
}

func TestOpenAIBatchImageProvider_SubmitWritesJSONLResult(t *testing.T) {
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(batchImageProviderTinyPNG) + `"}]}`)),
		}, nil
	})}

	provider := NewOpenAIBatchImageProvider(OpenAIBatchImageProviderOptions{ResultDir: t.TempDir()}, client)
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://openai.test"},
	}
	input := BatchImageInput{
		BatchID: "imgbatch_test",
		Model:   "gpt-image-2",
		Items:   []BatchImageInputItem{{CustomID: "item_1", Prompt: "draw"}},
	}

	job, err := provider.Submit(context.Background(), &BatchImageJob{BatchID: input.BatchID, Model: input.Model}, account, input)
	require.NoError(t, err)
	require.NotEmpty(t, job.ProviderOutputRef)

	r, _, err := provider.OpenResult(context.Background(), &BatchImageJob{ProviderOutputRef: &job.ProviderOutputRef}, account)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	line, err := findBatchImageLineImages(r, "item_1")
	require.NoError(t, err)
	require.Len(t, line.Images, 1)
	require.Equal(t, "image/png", line.Images[0].MimeType)
}

func TestMiniMaxBatchImageProvider_SubmitDownloadsImageURL(t *testing.T) {
	imageURL := "https://minimax.test/image.png"
	client := &http.Client{Transport: batchImageProviderRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/image_generation":
			require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"data":{"image_urls":["` + imageURL + `"]}}`)),
			}, nil
		case "/image.png":
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(bytes.NewReader(batchImageProviderTinyPNG)),
			}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
	})}

	provider := NewMiniMaxBatchImageProvider(MiniMaxBatchImageProviderOptions{ResultDir: t.TempDir()}, client)
	account := &Account{
		ID:          1,
		Platform:    PlatformMiniMax,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://minimax.test/v1"},
	}
	input := BatchImageInput{
		BatchID: "imgbatch_test",
		Model:   "image-01",
		Items:   []BatchImageInputItem{{CustomID: "item_1", Prompt: "draw"}},
	}

	job, err := provider.Submit(context.Background(), &BatchImageJob{BatchID: input.BatchID, Model: input.Model}, account, input)
	require.NoError(t, err)
	r, _, err := provider.OpenResult(context.Background(), &BatchImageJob{ProviderOutputRef: &job.ProviderOutputRef}, account)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	parsed, err := ParseBatchImageResultLine(data, 1)
	require.NoError(t, err)
	require.Equal(t, BatchImageParsedStatusSucceeded, parsed.Status)
	require.Equal(t, 1, parsed.ImageCount)
}
