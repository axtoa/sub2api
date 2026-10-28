//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestQualitySSECapture(t *testing.T) {
	for _, tt := range []struct {
		name, body           string
		ended                bool
		reason               string
		input, output, cache int64
		reported             bool
	}{
		{"responses", `data: {"type":"response.output_text.delta","delta":"hello"}
data: {"type":"response.completed","response":{"status":"completed","model":"real-model","usage":{"input_tokens":1200,"output_tokens":5,"input_tokens_details":{"cached_tokens":1024}}}}
`, true, "", 1200, 5, 1024, true},
		{"claude", `data: {"type":"message_start","message":{"model":"real-model","usage":{"input_tokens":12,"output_tokens":0,"cache_read_input_tokens":1024}}}
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}
data: {"type":"message_stop"}
`, true, "", 12, 5, 1024, true},
		{"chat", `data: {"model":"real-model","choices":[{"delta":{"content":"hello"},"finish_reason":null}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: {"choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":1024}}}
data: [DONE]
`, true, "", 1200, 5, 1024, true},
		{"length", `data: {"choices":[{"delta":{"content":"hello"},"finish_reason":"length"}]}
data: [DONE]
`, true, "truncated", 0, 0, 0, false},
		{"incomplete", `data: {"type":"response.output_text.delta","delta":"hello"}
data: {"type":"response.incomplete","response":{"status":"incomplete"}}
`, false, "incomplete", 0, 0, 0, false},
		{"prematureEOF", `data: {"type":"response.output_text.delta","delta":"hello"}`, false, "", 0, 0, 0, false},
		{"inputOnly", `data: {"type":"response.completed","response":{"usage":{"input_tokens":50}}}
`, true, "", 50, 0, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			capture := &qualityProbeCapture{start: time.Now()}
			body := &qualityProbeBody{ReadCloser: io.NopCloser(strings.NewReader(tt.body)), capture: capture}
			_, err := io.Copy(io.Discard, body)
			require.NoError(t, err)
			require.Equal(t, tt.ended, capture.ended)
			require.Equal(t, tt.reason, capture.result.Reason)
			require.Equal(t, tt.input, capture.result.InputTokens)
			require.Equal(t, tt.output, capture.result.OutputTokens)
			require.Equal(t, tt.cache, capture.result.CacheReadTokens)
			require.Equal(t, tt.reported, capture.result.UsageReported)
		})
	}
	capture := &qualityProbeCapture{start: time.Now()}
	body := &qualityProbeBody{ReadCloser: io.NopCloser(strings.NewReader(strings.Repeat("x", 140<<10))), capture: capture}
	_, err := io.Copy(io.Discard, body)
	require.Error(t, err)
	require.True(t, capture.limited)
}

func TestQualityProbeNoAccountMutations(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic} {
		for _, status := range []int{401, 403, 429, 500} {
			t.Run(platform+http.StatusText(status), func(t *testing.T) {
				account := &Account{ID: 1, Platform: platform, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "private-token", "base_url": "https://example.com"}}
				repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{1: account}}}
				upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(status, `{"error":"sensitive upstream detail"}`)}}
				svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
				result, err := svc.RunQualityProbe(context.Background(), 1, "text-model", QualityProbeOptions{Prompt: "hello", MaxOutputTokens: 256})
				require.NoError(t, err)
				require.False(t, result.Complete)
				require.Equal(t, status, result.HTTPStatus)
				require.NotContains(t, result.Reason, "sensitive")
				require.Zero(t, repo.setErrorID)
				require.Zero(t, repo.rateLimitedID)
				require.Nil(t, repo.updatedExtra)
				require.Nil(t, repo.bulkUpdatedIDs)
				require.Len(t, upstream.requests, 1)
			})
		}
	}
}

func TestQualityProbePinnedPayloadAndRawUsage(t *testing.T) {
	account := &Account{ID: 17, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Extra: map[string]any{"openai_responses_mode": "force_chat_completions"}, Credentials: map[string]any{"api_key": "private-token", "base_url": "https://example.com", "model_mapping": map[string]any{"alias": "real-model"}}}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{17: account}}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, `data: {"model":"real-model","choices":[{"delta":{"content":"answer"},"finish_reason":null}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: {"choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":1024}}}
data: [DONE]
`)}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	opts := QualityProbeOptions{Prompt: "question", Prefix: "stable prefix\n", MaxOutputTokens: 256, ReasoningEffort: "low"}
	result, err := svc.RunQualityProbe(context.Background(), 17, "alias", opts)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Equal(t, int64(1024), result.CacheReadTokens)
	require.Equal(t, int64(1200), result.InputTokens)
	require.True(t, result.UsageReported)
	require.Equal(t, "real-model", result.Model)
	require.NotNil(t, result.FirstTokenMs)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	raw, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "/v1/chat/completions", req.URL.Path)
	require.Equal(t, "real-model", gjson.GetBytes(raw, "model").String())
	require.Equal(t, "stable prefix\nquestion", gjson.GetBytes(raw, "messages.0.content").String())
	require.EqualValues(t, 256, gjson.GetBytes(raw, "max_completion_tokens").Int())
	require.Equal(t, "low", gjson.GetBytes(raw, "reasoning_effort").String())
	require.True(t, gjson.GetBytes(raw, "stream_options.include_usage").Bool())
	require.Greater(t, QualityProbeReservation(account.Platform, opts), int64(len(opts.Prompt)+len(opts.Prefix)+opts.MaxOutputTokens))
}

func TestQualityProbeRejectsUnsupportedWithoutRequest(t *testing.T) {
	require.False(t, QualityProbeSupported(nil, "x"))
	for _, model := range []string{"gpt-image-2", "embedding-model", "gpt-audio", "*"} {
		require.False(t, QualityProbeSupported(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, model))
	}
	require.False(t, QualityProbeSupported(&Account{Platform: PlatformGemini, Type: AccountTypeOAuth}, "text"))
	require.True(t, QualityProbeSupported(&Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}, "claude"))
}

type changingQualityAccountRepo struct {
	openAIAccountTestRepo
	reads int
	after *Account
}

func (r *changingQualityAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.reads++
	if r.reads > 1 {
		return r.after, nil
	}
	return r.openAIAccountTestRepo.GetByID(ctx, id)
}

func TestQualityProbeRechecksAccountBeforeRouting(t *testing.T) {
	for _, changed := range []*Account{
		{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey},
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"text-model": "gpt-image-2"}}},
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"auth_mode": "agentidentity"}},
	} {
		account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
		repo := &changingQualityAccountRepo{
			openAIAccountTestRepo: openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{1: account}}},
			after:                 changed,
		}
		upstream := &queuedHTTPUpstream{}
		svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
		result, err := svc.RunQualityProbe(context.Background(), 1, "text-model", QualityProbeOptions{Prompt: "hello", MaxOutputTokens: 256})
		require.NoError(t, err)
		require.False(t, result.Complete)
		require.Equal(t, "unsupported", result.Reason)
		require.Empty(t, upstream.requests)
		require.Zero(t, repo.setErrorID)
		require.Nil(t, repo.updatedExtra)
	}
}

func TestQualityProbeOAuthAndPayloadLimits(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "local-placeholder"}}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{1: account}}}
	response := newJSONResponse(200, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":20,\"output_tokens\":1}}}\n")
	response.Header.Set("x-codex-primary-used-percent", "90")
	upstream := &queuedHTTPUpstream{responses: []*http.Response{response}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	opts := QualityProbeOptions{Prompt: "custom question", Prefix: "prefix", CacheKey: "cache-key", MaxOutputTokens: 256, ReasoningEffort: "medium"}
	result, err := svc.RunQualityProbe(context.Background(), 1, "gpt-5", opts)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Nil(t, repo.updatedExtra)
	raw, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(raw, "max_output_tokens").Exists())
	require.True(t, result.NativeOutputLimit)
	require.Equal(t, "prefixcustom question", gjson.GetBytes(raw, "input.0.content.0.text").String())
	require.Equal(t, "cache-key", gjson.GetBytes(raw, "prompt_cache_key").String())
	require.Equal(t, "medium", gjson.GetBytes(raw, "reasoning.effort").String())
	capture := &qualityProbeCapture{options: opts}
	ctx := context.WithValue(context.Background(), qualityProbeContextKey{}, capture)
	payload := map[string]any{"system": "preserved"}
	applyQualityProbePayload(ctx, payload, "anthropic")
	require.Equal(t, 256, payload["max_tokens"])
	require.Equal(t, "preserved", payload["system"])
	require.NotContains(t, payload, "reasoning")
	content := payload["messages"].([]map[string]any)[0]["content"].([]map[string]any)
	require.Equal(t, "prefix", content[0]["text"])
	require.Equal(t, map[string]string{"type": "ephemeral"}, content[0]["cache_control"])
	require.Equal(t, "custom question", content[1]["text"])
	untouched := map[string]any{"existing": true}
	applyQualityProbePayload(context.Background(), untouched, "chat")
	require.Equal(t, map[string]any{"existing": true}, untouched)
	invalidUsage := &qualityProbeCapture{}
	invalidUsage.observe(`data: {"usage":{"input_tokens":"unknown","output_tokens":-1}}`)
	require.False(t, invalidUsage.result.UsageReported)
}

func TestQualityFinalTextAndDoneCompatibility(t *testing.T) {
	for _, stream := range []string{
		"data: {\"choices\":[{\"message\":{\"content\":\"42\"}}]}\ndata: [DONE]\n",
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"content\":[{\"type\":\"output_text\",\"text\":\"42\"}]}]}}\n",
	} {
		capture := &qualityProbeCapture{start: time.Now()}
		body := &qualityProbeBody{ReadCloser: io.NopCloser(strings.NewReader(stream)), capture: capture}
		_, err := io.Copy(io.Discard, body)
		require.NoError(t, err)
		require.True(t, capture.ended)
		require.Equal(t, "42", capture.result.Text)
		require.Empty(t, capture.result.Reason)
	}
	capture := &qualityProbeCapture{}
	capture.observeError([]byte(`{"error":{"param":"max_output_tokens","message":"private-token at https://private.example"}}`))
	require.Equal(t, "max_output_tokens", capture.result.ErrorParam)
	capture.observeError([]byte(`{"error":{"param":"secret-token"}}`))
	require.NotContains(t, capture.result.ErrorParam, "secret")
}
