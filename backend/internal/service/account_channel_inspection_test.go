//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestQualityChannelInspectionNonStreamAndToolResponses(t *testing.T) {
	for _, tc := range []struct {
		protocol, body, text string
		tools                bool
	}{
		{"chat", `{"model":"real-model","choices":[{"message":{"content":"nonce"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":2}}`, "nonce", false},
		{"responses", `{"object":"response","status":"completed","model":"real-model","output":[{"type":"message","content":[{"type":"output_text","text":"nonce"}]}],"usage":{"input_tokens":20,"output_tokens":2}}`, "nonce", false},
		{"anthropic", `{"type":"message","model":"real-model","content":[{"type":"text","text":"nonce"}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":2}}`, "nonce", false},
		{"chat", `{"model":"real-model","choices":[{"message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"channel_probe","arguments":"{\"nonce\":\"x\",\"value\":7}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":2}}`, "", true},
		{"responses", `{"object":"response","status":"completed","model":"real-model","output":[{"type":"function_call","call_id":"call-1","name":"channel_probe","arguments":"{\"nonce\":\"x\",\"value\":7}"}],"usage":{"input_tokens":20,"output_tokens":2}}`, "", true},
		{"anthropic", `{"type":"message","model":"real-model","content":[{"type":"tool_use","id":"call-1","name":"channel_probe","input":{"nonce":"x","value":7}}],"stop_reason":"tool_use","usage":{"input_tokens":20,"output_tokens":2}}`, "", true},
	} {
		t.Run(tc.protocol+tc.text+tc.body[1:8], func(t *testing.T) {
			p := &qualityProbeCapture{options: QualityProbeOptions{Inspection: true, NonStream: true}}
			p.result.Protocol = tc.protocol
			ctx := context.WithValue(context.Background(), qualityProbeContextKey{}, p)
			resp := newJSONResponse(200, tc.body)
			observeQualityProbeResponse(ctx, resp)
			handled, err := consumeChannelInspectionResponse(ctx, resp)
			require.True(t, handled)
			require.NoError(t, err)
			require.True(t, p.ended)
			require.False(t, p.wireInvalid)
			require.Empty(t, p.result.Reason)
			require.Equal(t, tc.text, p.result.Text)
			require.True(t, p.result.UsageReported)
			require.Nil(t, p.result.FirstTokenMs)
			if tc.tools {
				require.Len(t, p.result.ToolCalls, 1)
				require.Equal(t, "channel_probe", p.result.ToolCalls[0].Name)
			}
		})
	}
}

func TestQualityInspectionUsesExistingTransportAndPreservesLimits(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "local-placeholder", "base_url": "https://relay.example"}}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{1: account}}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, `{"object":"response","status":"completed","model":"real-model","output":[{"type":"message","content":[{"type":"output_text","text":"{\"nonce\":\"x\",\"value\":7}"}]}],"usage":{"input_tokens":20,"output_tokens":2}}`)}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	result, err := svc.RunQualityProbe(context.Background(), 1, "text-model", QualityProbeOptions{Inspection: true, NonStream: true, Structured: true, Prompt: "Return JSON", MaxOutputTokens: 256})
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.True(t, result.WireValid)
	raw, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(raw, "stream").Bool())
	require.EqualValues(t, 256, gjson.GetBytes(raw, "max_output_tokens").Int())
	require.Equal(t, "json_schema", gjson.GetBytes(raw, "text.format.type").String())
	require.Zero(t, repo.setErrorID)
	require.Nil(t, repo.updatedExtra)
}

func TestQualityInspectionToolFollowupAndProtocolFailures(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		payload := map[string]any{}
		applyChannelInspectionPayload(payload, protocol, QualityProbeOptions{Inspection: true, NonStream: true, ToolCallID: "call-1", ToolArguments: `{"nonce":"x","value":7}`, ToolResult: "receipt", Prompt: "Call tool then echo receipt"})
		require.NotContains(t, payload, "tool_choice")
		require.Contains(t, payload, "tools")
	}
	p := &qualityProbeCapture{options: QualityProbeOptions{Inspection: true, NonStream: true}}
	p.result.Protocol = "chat"
	ctx := context.WithValue(context.Background(), qualityProbeContextKey{}, p)
	resp := newJSONResponse(200, "data: [DONE]\n")
	observeQualityProbeResponse(ctx, resp)
	_, err := consumeChannelInspectionResponse(ctx, resp)
	require.NoError(t, err)
	require.Equal(t, "protocol_invalid", p.result.Reason)
	p = &qualityProbeCapture{options: QualityProbeOptions{Inspection: true}}
	p.result.Protocol = "chat"
	p.observe(`data: {"choices":[{"delta":{"content":"x"},"finish_reason":"length"}]}`)
	p.observe("data: [DONE]")
	require.Equal(t, "truncated", p.result.Reason)
}

func TestQualityInspectionStreamingOrderAndVisualPayloads(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		payload := map[string]any{}
		applyChannelInspectionPayload(payload, protocol, QualityProbeOptions{Inspection: true, Prompt: "Count squares", ImageDataURL: "data:image/png;base64,ZmFrZQ=="})
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		require.Contains(t, string(raw), "ZmFrZQ==")
	}
	p := &qualityProbeCapture{options: QualityProbeOptions{Inspection: true}}
	p.result.Protocol = "anthropic"
	p.observe(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"x"}}`)
	p.observe(`data: {"type":"message_stop"}`)
	require.True(t, p.wireInvalid)
	p = &qualityProbeCapture{options: QualityProbeOptions{Inspection: true}}
	p.result.Protocol = "chat"
	p.observe(`data: {"choices":[{"delta":{"content":"x"}}]}`)
	p.observe(`data: [DONE]`)
	require.True(t, p.wireInvalid)
}
