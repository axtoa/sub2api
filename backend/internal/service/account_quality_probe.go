package service

import (
	"bytes"
	"context"
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
	Prompt          string
	Prefix          string
	CacheKey        string
	ReasoningEffort string
	MaxOutputTokens int
}

type QualityProbeResult struct {
	Text             string `json:"text"`
	Model            string `json:"model"`
	HTTPStatus       int    `json:"http_status"`
	Complete         bool   `json:"complete"`
	Reason           string `json:"reason"`
	DurationMs       int64  `json:"duration_ms"`
	FirstTokenMs     *int64 `json:"first_token_ms"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	UsageReported    bool   `json:"usage_reported"`
	CacheReported    bool   `json:"cache_reported"`
}

type qualityProbeCapture struct {
	options      QualityProbeOptions
	result       QualityProbeResult
	start        time.Time
	ended        bool
	limited      bool
	inputSeen    bool
	outputSeen   bool
	usageInvalid bool
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
	return int64(len(system) + len(opts.Prompt) + len(opts.Prefix) + opts.MaxOutputTokens + 1024)
}

func (s *AccountTestService) RunQualityProbe(ctx context.Context, accountID int64, model string, opts QualityProbeOptions) (*QualityProbeResult, error) {
	if model == "" || len(model) > 100 || len(opts.Prompt)+len(opts.Prefix) > 100000 || opts.MaxOutputTokens < 64 || opts.MaxOutputTokens > 8192 {
		return nil, fmt.Errorf("invalid quality probe")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account unavailable")
	}
	if !QualityProbeSupported(account, model) {
		return &QualityProbeResult{Reason: "unsupported"}, nil
	}
	capture := &qualityProbeCapture{options: opts, start: time.Now()}
	ctx = context.WithValue(ctx, qualityProbeContextKey{}, capture)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = (&http.Request{Header: make(http.Header)}).WithContext(ctx)
	err = s.TestAccountConnection(c, accountID, model, opts.Prompt, AccountTestModeDefault)
	r := &capture.result
	r.DurationMs = time.Since(capture.start).Milliseconds()
	r.Complete = err == nil && capture.ended && !capture.limited && strings.TrimSpace(r.Text) != "" && r.Reason == ""
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
	if a.Type != AccountTypeAPIKey && a.Type != AccountTypeOAuth && !(a.Platform == PlatformAnthropic && a.Type == AccountTypeSetupToken) {
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
		payload["max_output_tokens"] = o.MaxOutputTokens
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
	capture *qualityProbeCapture
	pending []byte
	total   int
}

func (b *qualityProbeBody) Read(dst []byte) (int, error) {
	if b.total >= 1<<20 {
		b.capture.limited = true
		return 0, io.ErrUnexpectedEOF
	}
	n, err := b.ReadCloser.Read(dst)
	b.total += n
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
	if !gjson.Valid(raw) {
		return
	}
	v := gjson.Parse(raw)
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
	case "response.incomplete", "response.failed", "error":
		p.result.Reason = "incomplete"
	}
	for _, choice := range v.Get("choices").Array() {
		text += choice.Get("delta.content").String()
		finish := choice.Get("finish_reason").String()
		if finish != "" {
			p.ended = true
			if finish != "stop" {
				p.result.Reason = "incomplete"
			}
		}
	}
	if stop := v.Get("delta.stop_reason").String(); stop != "" && stop != "end_turn" && stop != "stop_sequence" {
		p.result.Reason = "incomplete"
	}
	if status := v.Get("response.status").String(); status != "" && status != "completed" && p.ended {
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
