// Package quality implements account-pinned administrator quality monitoring.
package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const GeneratorVersion = "objective-v1"

type Profile struct {
	AvailabilityEnabled         bool   `json:"availability_enabled"`
	AvailabilityIntervalMinutes int    `json:"availability_interval_minutes"`
	QualityEnabled              bool   `json:"quality_enabled"`
	QualityIntervalMinutes      int    `json:"quality_interval_minutes"`
	CacheEnabled                bool   `json:"cache_enabled"`
	CacheIntervalMinutes        int    `json:"cache_interval_minutes"`
	Questions                   int    `json:"questions"`
	PassPercent                 int    `json:"pass_percent"`
	RetryCount                  int    `json:"retry_count"`
	RetryIntervalMinutes        int    `json:"retry_interval_minutes"`
	MaxOutputTokens             int    `json:"max_output_tokens"`
	ReasoningEffort             string `json:"reasoning_effort"`
	CachePrefixKB               int    `json:"cache_prefix_kb"`
}

type Settings struct {
	Profile
	AutoEnabled   bool  `json:"auto_enabled"`
	DailyRequests int   `json:"daily_requests"`
	DailyTokens   int64 `json:"daily_tokens"`
	Version       int   `json:"version"`
}

func DefaultSettings() Settings {
	return Settings{Profile: Profile{
		AvailabilityEnabled: true, AvailabilityIntervalMinutes: 60,
		QualityEnabled: true, QualityIntervalMinutes: 720,
		CacheIntervalMinutes: 1440, Questions: 3, PassPercent: 80,
		RetryCount: 1, RetryIntervalMinutes: 5, MaxOutputTokens: 2048, CachePrefixKB: 32,
	}, AutoEnabled: true, DailyRequests: 1000, DailyTokens: 2000000, Version: 1}
}

func invalid(message string) error  { return apperrors.BadRequest("QUALITY_INVALID", message) }
func conflict(message string) error { return apperrors.Conflict("QUALITY_CONFLICT", message) }

func (p Profile) Validate() error {
	if p.AvailabilityIntervalMinutes < 1 || p.AvailabilityIntervalMinutes > 10080 ||
		p.QualityIntervalMinutes < 5 || p.QualityIntervalMinutes > 10080 ||
		p.CacheIntervalMinutes < 5 || p.CacheIntervalMinutes > 10080 ||
		p.Questions < 3 || p.Questions > 8 || p.PassPercent < 50 || p.PassPercent > 100 ||
		p.RetryCount < 0 || p.RetryCount > 3 || p.RetryIntervalMinutes < 1 || p.RetryIntervalMinutes > 1440 ||
		p.MaxOutputTokens < 256 || p.MaxOutputTokens > 8192 || p.CachePrefixKB < 8 || p.CachePrefixKB > 64 {
		return invalid("检测间隔、题数、复测次数或输出上限超出允许范围")
	}
	switch p.ReasoningEffort {
	case "", "low", "medium", "high":
		return nil
	default:
		return invalid("推理强度无效")
	}
}

func (s Settings) Validate() error {
	if s.DailyRequests < 1 || s.DailyRequests > 100000 || s.DailyTokens < 1000 || s.DailyTokens > 1000000000 {
		return invalid("请设置有效的每日请求与 token 预算")
	}
	return s.Profile.Validate()
}

type Target struct {
	ID                 string     `json:"id"`
	AccountID          int64      `json:"account_id"`
	AccountName        string     `json:"account_name"`
	Platform           string     `json:"platform"`
	AccountStatus      string     `json:"account_status"`
	Model              string     `json:"model"`
	Enabled            bool       `json:"enabled"`
	Overrides          *Profile   `json:"overrides"`
	Version            int        `json:"version"`
	NextAvailabilityAt time.Time  `json:"next_availability_at"`
	NextQualityAt      time.Time  `json:"next_quality_at"`
	NextCacheAt        time.Time  `json:"next_cache_at"`
	LastAvailabilityAt *time.Time `json:"last_availability_at"`
	LastQualityAt      *time.Time `json:"last_quality_at"`
	LastCacheAt        *time.Time `json:"last_cache_at"`
	RetryAttempt       int        `json:"retry_attempt"`
	Metrics            Metrics    `json:"metrics"`
	Recent             []Run      `json:"recent"`
	LatestQuality      *Run       `json:"latest_quality"`
	LatestCache        *Run       `json:"latest_cache"`
	Pending            *Run       `json:"pending"`
	AccountEnabled     bool       `json:"account_enabled"`
	ProbeSuccesses     int        `json:"probe_successes"`
	ProbeFailures      int        `json:"probe_failures"`
}

func (t Target) Effective(s Settings) Profile {
	if t.Overrides != nil {
		return *t.Overrides
	}
	return s.Profile
}

type Metrics struct {
	Requests          int64      `json:"requests"`
	Successes         int64      `json:"successes"`
	Errors            int64      `json:"errors"`
	FirstTokenMs      *float64   `json:"first_token_ms"`
	BilledInputTokens int64      `json:"billed_input_tokens"`
	BilledCacheTokens int64      `json:"billed_cache_tokens"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
}

type Run struct {
	ID          string     `json:"id"`
	TargetID    string     `json:"target_id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Manual      bool       `json:"manual"`
	Verdict     string     `json:"verdict"`
	ProfileHash string     `json:"profile_hash"`
	Profile     *Profile   `json:"profile,omitempty"`
	Result      *Result    `json:"result,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

type Result struct {
	Reason           string   `json:"reason"`
	Correct          int      `json:"correct"`
	Graded           int      `json:"graded"`
	Planned          int      `json:"planned"`
	Requests         int      `json:"requests"`
	BudgetTokens     int64    `json:"budget_tokens"`
	FirstTokenMs     *int64   `json:"first_token_ms"`
	DurationMs       int64    `json:"duration_ms"`
	Items            []Sample `json:"items"`
	CachePrefix      string   `json:"cache_prefix,omitempty"`
	CacheReported    bool     `json:"cache_reported"`
	CacheInputTokens int64    `json:"cache_input_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
}

type Sample struct {
	QuestionID  string                      `json:"question_id"`
	Family      string                      `json:"family"`
	Prompt      string                      `json:"prompt"`
	Expected    string                      `json:"expected"`
	Explanation string                      `json:"explanation"`
	Correct     *bool                       `json:"correct"`
	Probe       *service.QualityProbeResult `json:"probe"`
}

type Budget struct {
	Day                 string `json:"day"`
	Timezone            string `json:"timezone"`
	Requests            int    `json:"requests"`
	Tokens              int64  `json:"tokens"`
	UnconfirmedRequests int    `json:"unconfirmed_requests"`
	Limited             bool   `json:"limited"`
	ManualRequests      int    `json:"manual_requests"`
	ManualTokens        int64  `json:"manual_tokens"`
	ManualUnconfirmed   int    `json:"manual_unconfirmed"`
}

func validKind(kind string) bool {
	return kind == "availability" || kind == "quality" || kind == "cache"
}

func profileHash(a *service.Account, t Target, p Profile) string {
	// A route/proxy/profile change invalidates comparison with old results.
	proxyURL := ""
	if a.Proxy != nil {
		proxyURL = a.Proxy.URL()
	}
	data, _ := json.Marshal([]any{GeneratorVersion, a.ID, t.Model, a.GetMappedModel(t.Model), a.Platform, a.Type, a.ProxyID, proxyURL, a.GetBaseURL(), a.GetOpenAIBaseURL(), openai_compat.ShouldUseResponsesAPI(a.Extra), a.GetHeaderOverrides(), p})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func validateModel(model string) error {
	if strings.TrimSpace(model) != model || model == "" || len(model) > 100 || strings.ContainsAny(model, "*?\r\n\t") {
		return invalid("请填写具体的文本模型名称，不支持通配符")
	}
	return nil
}

func period(p Profile, kind string) time.Duration {
	minutes := p.AvailabilityIntervalMinutes
	if kind == "quality" {
		minutes = p.QualityIntervalMinutes
	} else if kind == "cache" {
		minutes = p.CacheIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func safeReason(err error) string {
	if err == nil {
		return ""
	}
	// DB/network errors must not disclose connection strings or account secrets.
	return fmt.Sprintf("monitor_error:%T", err)
}
