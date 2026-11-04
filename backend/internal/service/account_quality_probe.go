package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Quality probes reuse the account test transport, model mapping, proxy and
// authentication. The private context capability never comes from HTTP input.
// It disables account-state mutations and captures usage BEFORE billing rules.
type qualityProbeContextKey struct{}

type QualityProbeOptions struct {
	Audit             bool
	AuditNative       bool
	ImageDataURL      string
	Inspection        bool
	NonStream         bool
	Structured        bool
	Tools             bool
	ToolCallID        string
	ToolArguments     string
	ToolResult        string
	Prompt            string
	Prefix            string
	CacheKey          string
	ReasoningEffort   string
	MaxOutputTokens   int
	NativeOutputLimit bool
}

type QualityToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type QualityProbeResult struct {
	UsageAnomalies    []string          `json:"usage_anomalies,omitempty"`
	RequestHash       string            `json:"request_hash,omitempty"`
	ControlledInput   bool              `json:"controlled_input"`
	RequestBytes      int               `json:"request_bytes,omitempty"`
	ReasoningTokens   *int64            `json:"reasoning_tokens"`
	TotalTokens       *int64            `json:"total_tokens"`
	Protocol          string            `json:"protocol,omitempty"`
	WireValid         bool              `json:"wire_valid"`
	FinishReason      string            `json:"finish_reason,omitempty"`
	ToolCalls         []QualityToolCall `json:"tool_calls,omitempty"`
	Text              string            `json:"text"`
	Model             string            `json:"model"`
	HTTPStatus        int               `json:"http_status"`
	Complete          bool              `json:"complete"`
	Reason            string            `json:"reason"`
	ErrorParam        string            `json:"error_param,omitempty"`
	NativeOutputLimit bool              `json:"native_output_limit,omitempty"`
	DurationMs        int64             `json:"duration_ms"`
	FirstTokenMs      *int64            `json:"first_token_ms"`
	InputTokens       int64             `json:"input_tokens"`
	OutputTokens      int64             `json:"output_tokens"`
	CacheReadTokens   int64             `json:"cache_read_tokens"`
	CacheWriteTokens  int64             `json:"cache_write_tokens"`
	UsageReported     bool              `json:"usage_reported"`
	CacheReported     bool              `json:"cache_reported"`
}

type qualityProbeCapture struct {
	options            QualityProbeOptions
	result             QualityProbeResult
	start              time.Time
	ended              bool
	limited            bool
	inputSeen          bool
	outputSeen         bool
	usageInvalid       bool
	wireInvalid        bool
	observedEvents     int
	inspectionStarted  bool
	inspectionTerminal bool
}

func qualityCapture(ctx context.Context) *qualityProbeCapture {
	p, _ := ctx.Value(qualityProbeContextKey{}).(*qualityProbeCapture)
	return p
}

func isQualityProbe(ctx context.Context) bool { return qualityCapture(ctx) != nil }

// QualityProbeReservation is a conservative UTF-8 byte bound for input tokens,
// plus the enforced output limit. Missing/incomplete usage keeps this reserved
// amount charged to the monitor budget rather than silently costing zero.
func QualityProbeReservation(platform string, opts QualityProbeOptions) int64 {
	system := openai.DefaultInstructions
	if platform == PlatformAnthropic {
		system = claudeCodeSystemPrompt
	}
	outputReservation := opts.MaxOutputTokens
	if opts.NativeOutputLimit {
		outputReservation = max(outputReservation, 32768)
	}
	imageReservation := 0
	if opts.ImageDataURL != "" {
		imageReservation = 4096
	}
	return int64(imageReservation + len(system) + len(opts.Prompt) + len(opts.Prefix) + outputReservation + 1024)
}

func (s *AccountTestService) RunQualityProbe(ctx context.Context, accountID int64, model string, opts QualityProbeOptions) (*QualityProbeResult, error) {
	if model == "" || len(model) > 100 || len(opts.Prompt)+len(opts.Prefix)+len(opts.ToolArguments)+len(opts.ToolResult)+len(opts.ImageDataURL) > 100000 || opts.MaxOutputTokens < 64 || opts.MaxOutputTokens > 8192 {
		return nil, fmt.Errorf("invalid quality probe")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account unavailable")
	}
	if !QualityProbeSupported(account, model) {
		return &QualityProbeResult{Reason: "unsupported"}, nil
	}
	opts.NativeOutputLimit = account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth
	opts.AuditNative = account.Type != AccountTypeAPIKey
	capture := &qualityProbeCapture{options: opts, start: time.Now()}
	capture.result.NativeOutputLimit = opts.NativeOutputLimit
	ctx = context.WithValue(ctx, qualityProbeContextKey{}, capture)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = (&http.Request{Header: make(http.Header)}).WithContext(ctx)
	err = s.TestAccountConnection(c, accountID, model, opts.Prompt, AccountTestModeDefault)
	r := &capture.result
	if capture.usageInvalid {
		r.UsageAnomalies = append(r.UsageAnomalies, "invalid_token_count")
	}
	if r.UsageReported {
		if r.ReasoningTokens != nil && *r.ReasoningTokens > r.OutputTokens {
			r.UsageAnomalies = append(r.UsageAnomalies, "reasoning_exceeds_output")
		}
		if r.Protocol != "anthropic" && r.CacheReadTokens > r.InputTokens {
			r.UsageAnomalies = append(r.UsageAnomalies, "cache_exceeds_input")
		}
		if r.Protocol != "anthropic" && r.TotalTokens != nil && *r.TotalTokens != r.InputTokens+r.OutputTokens {
			r.UsageAnomalies = append(r.UsageAnomalies, "total_inconsistent")
		}
	}
	r.DurationMs = time.Since(capture.start).Milliseconds()
	r.WireValid = capture.observedEvents > 0 && !capture.wireInvalid && capture.ended
	r.Complete = err == nil && capture.ended && !capture.limited && (strings.TrimSpace(r.Text) != "" || (opts.Inspection && len(r.ToolCalls) > 0)) && r.Reason == ""
	if !r.Complete && r.Reason == "" {
		switch {
		case ctx.Err() != nil:
			r.Reason = "timeout"
		case r.HTTPStatus == 401 || r.HTTPStatus == 403:
			r.Reason = "authentication"
		case r.HTTPStatus == 429:
			r.Reason = "rate_limited"
		case r.HTTPStatus == 400 || r.HTTPStatus == 404 || r.HTTPStatus == 422:
			r.Reason = "request_rejected"
		case r.HTTPStatus >= 500:
			r.Reason = "upstream_error"
		case r.HTTPStatus == 0:
			r.Reason = "transport_error"
		case capture.limited:
			r.Reason = "response_too_large"
		case capture.ended && strings.TrimSpace(r.Text) == "":
			r.Reason = "empty_response"
		default:
			r.Reason = "incomplete"
		}
	}
	return r, nil
}

func QualityProbeSupported(a *Account, model string) bool {
	if a == nil || a.IsSyntheticUITest() || (a.Platform != PlatformOpenAI && a.Platform != PlatformAnthropic) {
		return false
	}
	// Agent Identity may create/replace remote tasks during authentication.
	// Credential shadows resolve another account, so cannot promise a pinned
	// account/configuration in this first observation-only version.
	if a.IsOpenAIAgentIdentity() || a.IsCredentialShadow() {
		return false
	}
	if a.Type != AccountTypeAPIKey && a.Type != AccountTypeOAuth && (a.Platform != PlatformAnthropic || a.Type != AccountTypeSetupToken) {
		return false
	}
	m := strings.ToLower(a.GetMappedModel(model))
	for _, keyword := range []string{"image", "embedding", "audio", "realtime", "whisper", "tts", "moderation", "video"} {
		if strings.Contains(m, keyword) {
			return false
		}
	}
	return strings.TrimSpace(m) != "" && !strings.ContainsAny(m, "*?\r\n")
}

func applyQualityProbePayload(ctx context.Context, payload map[string]any, protocol string) {
	p := qualityCapture(ctx)
	if p == nil {
		return
	}
	o := p.options
	if o.Audit {
		p.result.ControlledInput = false
	}
	p.result.Protocol = protocol
	prompt := o.Prompt
	switch protocol {
	case "anthropic":
		content := []map[string]any{}
		if o.Prefix != "" {
			content = append(content, map[string]any{"type": "text", "text": o.Prefix, "cache_control": map[string]string{"type": "ephemeral"}})
		}
		content = append(content, map[string]any{"type": "text", "text": prompt})
		payload["messages"] = []map[string]any{{"role": "user", "content": content}}
		payload["max_tokens"] = o.MaxOutputTokens
		// Keep the same controlled profile across accounts. Thinking is left at
		// the native default; OpenAI-only effort values are never sent to Claude.
	case "responses":
		payload["input"] = []map[string]any{{"role": "user", "content": []map[string]any{{"type": "input_text", "text": o.Prefix + prompt}}}}
		// Codex OAuth uses the native response limit, like the existing connection test.
		// Its backend rejects max_output_tokens; API-key upstreams retain the explicit cap.
		if !o.NativeOutputLimit {
			payload["max_output_tokens"] = o.MaxOutputTokens
		}
		if o.ReasoningEffort != "" {
			payload["reasoning"] = map[string]string{"effort": o.ReasoningEffort}
		}
		if o.CacheKey != "" {
			payload["prompt_cache_key"] = o.CacheKey
		}
	case "chat":
		payload["messages"] = []map[string]any{{"role": "user", "content": o.Prefix + prompt}}
		payload["max_completion_tokens"] = o.MaxOutputTokens
		payload["stream_options"] = map[string]bool{"include_usage": true}
		if o.ReasoningEffort != "" {
			payload["reasoning_effort"] = o.ReasoningEffort
		}
	}
	if o.Inspection {
		applyChannelInspectionPayload(payload, protocol, o)
	}
	if o.Audit && !o.NativeOutputLimit && !o.AuditNative {
		// Remove our connection-test persona from controlled comparisons. Native
		// OAuth instructions remain untouched and are explicitly noncomparable.
		const instruction = "Answer the user's verification task exactly."
		switch protocol {
		case "anthropic":
			payload["system"] = instruction
		case "responses":
			payload["instructions"] = instruction
		}
		p.result.ControlledInput = true
	}
	// This is the final body at all three account-test call sites. Headers and
	// credentials are deliberately excluded; only synthetic request bytes hash.
	if o.Audit {
		raw, err := json.Marshal(payload)
		if err == nil {
			p.result.RequestHash = fmt.Sprintf("%x", sha256.Sum256(raw))
			p.result.RequestBytes = len(raw)
		}
	}
}

func observeQualityProbeResponse(ctx context.Context, resp *http.Response) {
	p := qualityCapture(ctx)
	if p == nil {
		return
	}
	p.result.HTTPStatus = resp.StatusCode
	resp.Body = &qualityProbeBody{ReadCloser: resp.Body, capture: p}
}

type qualityProbeBody struct {
	io.ReadCloser
	capture   *qualityProbeCapture
	pending   []byte
	errorBody []byte
	total     int
}

func (b *qualityProbeBody) Read(dst []byte) (int, error) {
	if b.total >= 1<<20 {
		b.capture.limited = true
		return 0, io.ErrUnexpectedEOF
	}
	n, err := b.ReadCloser.Read(dst)
	b.total += n
	if b.capture.result.HTTPStatus >= 400 && len(b.errorBody) < 16384 {
		b.errorBody = append(b.errorBody, dst[:min(n, 16384-len(b.errorBody))]...)
		b.capture.observeError(b.errorBody)
	}
	b.pending = append(b.pending, dst[:n]...)
	for {
		idx := bytes.IndexByte(b.pending, '\n')
		if idx < 0 {
			break
		}
		b.capture.observe(strings.TrimSpace(string(b.pending[:idx])))
		b.pending = b.pending[idx+1:]
	}
	if len(b.pending) > 128<<10 {
		b.capture.limited = true
		return n, io.ErrUnexpectedEOF
	}
	if err == io.EOF && len(b.pending) > 0 {
		b.capture.observe(strings.TrimSpace(string(b.pending)))
		b.pending = nil
	}
	return n, err
}

func (p *qualityProbeCapture) observe(line string) {
	if !strings.HasPrefix(line, "data:") {
		return
	}
	raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if raw == "[DONE]" {
		if p.options.Inspection && p.result.Protocol == "chat" && !p.inspectionTerminal {
			p.wireInvalid = true
		}
		p.ended = true
		return
	}
	if !gjson.Valid(raw) {
		p.wireInvalid = true
		return
	}
	v := gjson.Parse(raw)
	if p.options.Inspection {
		p.observeInspection(v)
	}
	event := v.Get("type").String()
	if model := v.Get("model").String(); model != "" {
		p.result.Model = model
	}
	for _, path := range []string{"message.model", "response.model"} {
		if model := v.Get(path).String(); model != "" {
			p.result.Model = model
		}
	}
	text := ""
	switch event {
	case "content_block_delta":
		text = v.Get("delta.text").String()
	case "response.output_text.delta":
		text = v.Get("delta").String()
	case "message_stop", "response.completed", "response.done":
		p.ended = true
		if p.result.Text == "" {
			for _, item := range v.Get("response.output").Array() {
				for _, part := range item.Get("content").Array() {
					if part.Get("type").String() == "output_text" {
						text += part.Get("text").String()
					}
				}
			}
		}
	case "response.incomplete", "response.failed", "error":
		p.result.Reason = "incomplete"
		if v.Get("response.incomplete_details.reason").String() == "max_output_tokens" {
			p.result.Reason = "truncated"
		}
		p.observeError([]byte(raw))
	}
	for _, choice := range v.Get("choices").Array() {
		delta := choice.Get("delta.content").String()
		text += delta
		if delta == "" && p.result.Text == "" {
			text += choice.Get("message.content").String()
		}
		finish := choice.Get("finish_reason").String()
		if finish != "" {
			p.ended = true
			p.result.FinishReason = finish
			if finish != "stop" && !(p.options.Inspection && finish == "tool_calls") {
				p.result.Reason = "incomplete"
				if finish == "length" {
					p.result.Reason = "truncated"
				}
				if finish == "content_filter" {
					p.result.Reason = "content_filtered"
				}
			}
		}
	}
	if stop := v.Get("delta.stop_reason").String(); stop != "" {
		p.result.FinishReason = stop
		if stop != "end_turn" && stop != "stop_sequence" && !(p.options.Inspection && stop == "tool_use") {
			p.result.Reason = "incomplete"
			if stop == "max_tokens" {
				p.result.Reason = "truncated"
			}
		}
	}
	if status := v.Get("response.status").String(); status != "" && status != "completed" && p.ended && p.result.Reason == "" {
		p.result.Reason = "incomplete"
	}
	if text != "" {
		if p.result.FirstTokenMs == nil {
			ms := time.Since(p.start).Milliseconds()
			p.result.FirstTokenMs = &ms
		}
		if len(p.result.Text)+len(text) > 32000 {
			p.limited = true
		} else {
			p.result.Text += text
		}
	}
	for _, path := range []string{"usage", "message.usage", "response.usage"} {
		u := v.Get(path)
		if !u.IsObject() {
			continue
		}
		for _, key := range []string{"input_tokens", "prompt_tokens"} {
			if n := u.Get(key); n.Exists() {
				p.inputSeen = true
				p.result.InputTokens = qualityMaxTokens(p.result.InputTokens, p.usageToken(n))
			}
		}
		for _, key := range []string{"output_tokens", "completion_tokens"} {
			if n := u.Get(key); n.Exists() {
				p.outputSeen = true
				p.result.OutputTokens = qualityMaxTokens(p.result.OutputTokens, p.usageToken(n))
			}
		}
		for _, key := range []string{"cache_read_input_tokens", "input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens", "cached_tokens"} {
			if n := u.Get(key); n.Exists() {
				p.result.CacheReported = true
				p.result.CacheReadTokens = qualityMaxTokens(p.result.CacheReadTokens, p.usageToken(n))
			}
		}
		if n := u.Get("cache_creation_input_tokens"); n.Exists() {
			p.result.CacheWriteTokens = qualityMaxTokens(p.result.CacheWriteTokens, p.usageToken(n))
		}
		for _, key := range []string{"output_tokens_details.reasoning_tokens", "completion_tokens_details.reasoning_tokens"} {
			if n := u.Get(key); n.Exists() {
				value := p.usageToken(n)
				if p.result.ReasoningTokens == nil || value > *p.result.ReasoningTokens {
					p.result.ReasoningTokens = &value
				}
			}
		}
		if n := u.Get("total_tokens"); n.Exists() {
			value := p.usageToken(n)
			if p.result.TotalTokens == nil || value > *p.result.TotalTokens {
				p.result.TotalTokens = &value
			}
		}
		p.result.UsageReported = p.inputSeen && p.outputSeen && !p.usageInvalid
	}
}

func (p *qualityProbeCapture) usageToken(value gjson.Result) int64 {
	n, err := strconv.ParseInt(value.Raw, 10, 64)
	if err != nil || value.Type != gjson.Number || n < 0 || n > 1000000000 {
		p.usageInvalid = true
		return 0
	}
	return n
}

// Probe errors deliberately omit upstream bodies, URLs and credentials from
// logs/history. The structured status is sufficient for monitoring.
func qualityProbeError(ctx context.Context) error {
	p := qualityCapture(ctx)
	if p == nil {
		return nil
	}
	return fmt.Errorf("quality probe did not complete (HTTP %d)", p.result.HTTPStatus)
}

func qualityMaxTokens(a, b int64) int64 {
	if b > a {
		return b
	}
	return a
}

// Keep diagnostics actionable without storing upstream text, URLs or credentials.
func (p *qualityProbeCapture) observeError(raw []byte) {
	if !gjson.ValidBytes(raw) {
		return
	}
	param := gjson.GetBytes(raw, "error.param").String()
	if param == "" {
		param = openAIResponsesRejectedParamFromMessage(strings.ToLower(gjson.GetBytes(raw, "error.message").String()))
	}
	switch param {
	case "model", "max_output_tokens", "max_completion_tokens", "max_tokens", "stream_options", "stream_options.include_usage", "reasoning", "reasoning.effort", "reasoning_effort", "prompt_cache_key", "tools", "tool_choice", "response_format", "text.format", "output_config", "stream":
		p.result.ErrorParam = param
	}
}
