package quality

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var scoreDimensions = []string{"availability", "capability", "compatibility", "latency", "integrity", "billing"}

type ScorePolicy struct {
	BillingTolerancePercent int            `json:"billing_tolerance_percent"`
	MinSpanMinutes          int            `json:"min_span_minutes"`
	Scenario                string         `json:"scenario"`
	Weights                 map[string]int `json:"weights"`
	MinSamples              int            `json:"min_samples"`
	MinRuns                 int            `json:"min_runs"`
	MinCoverage             int            `json:"min_coverage"`
	FreshHours              int            `json:"fresh_hours"`
	SlowMS                  int64          `json:"slow_ms"`
	FailurePercent          int            `json:"failure_percent"`
	UsageTolerancePercent   int            `json:"usage_tolerance_percent"`
	RequiredChecks          []string       `json:"required_checks"`
}

type AuditReference struct {
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	Name     string `json:"name"`
	Source   string `json:"source"` // administrator-labelled official / trusted / other
}

type AssuranceSettings struct {
	Version          int                    `json:"version"`
	Policy           ScorePolicy            `json:"policy"`
	References       []AuditReference       `json:"references"`
	ManualReferences []ManualTokenReference `json:"manual_references"`
}

// Counts copied from a counting endpoint or trusted ledger are assertions by
// the administrator. Exact request hashes prevent applying a count to new input.
type ManualTokenReference struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Model       string    `json:"model"`
	Protocol    string    `json:"protocol"`
	RequestHash string    `json:"request_hash"`
	InputTokens int64     `json:"input_tokens"` // total input, including Claude cache components
	CapturedAt  time.Time `json:"captured_at"`
}

func DefaultScorePolicy(scenario string) ScorePolicy {
	p := ScorePolicy{Scenario: scenario, Weights: map[string]int{"availability": 25, "capability": 25, "compatibility": 20, "latency": 10, "integrity": 10, "billing": 10}, MinSamples: 8, MinRuns: 3, MinCoverage: 60, FreshHours: 48, SlowMS: 5000, FailurePercent: 20, UsageTolerancePercent: 15, RequiredChecks: []string{"protocol_stream"}}
	if scenario == "agent" {
		p.Weights = map[string]int{"availability": 25, "capability": 20, "compatibility": 30, "latency": 5, "integrity": 10, "billing": 10}
		p.RequiredChecks = []string{"protocol_stream", "tools", "structured", "context"}
	}
	if scenario == "cost" {
		p.Weights = map[string]int{"availability": 20, "capability": 20, "compatibility": 10, "latency": 5, "integrity": 20, "billing": 25}
		p.RequiredChecks = []string{"protocol_stream", "usage_audit"}
	}
	p.MinSpanMinutes = 60
	p.BillingTolerancePercent = 2
	return p
}

func (s AssuranceSettings) validate() error {
	p := s.Policy
	if p.BillingTolerancePercent < 0 || p.BillingTolerancePercent > 100 || p.MinSpanMinutes < 0 || p.MinSpanMinutes > 1440 || s.Version < 1 || (p.Scenario != "chat" && p.Scenario != "agent" && p.Scenario != "cost") || len(p.Weights) != len(scoreDimensions) || p.MinSamples < 5 || p.MinSamples > 100 || p.MinRuns < 2 || p.MinRuns > 20 || p.MinCoverage < 30 || p.MinCoverage > 100 || p.FreshHours < 1 || p.FreshHours > 168 || p.SlowMS < 100 || p.SlowMS > 120000 || p.FailurePercent < 5 || p.FailurePercent > 80 || p.UsageTolerancePercent < 1 || p.UsageTolerancePercent > 100 {
		return invalid("评分规则超出允许范围")
	}
	total := 0
	for _, d := range scoreDimensions {
		w, ok := p.Weights[d]
		if !ok || w < 1 || w > 80 {
			return invalid("六个维度的权重必须为正数")
		}
		total += w
	}
	if total != 100 {
		return invalid("评分权重合计必须为 100")
	}
	seen := map[string]bool{}
	for _, id := range p.RequiredChecks {
		valid := false
		for _, known := range inspectionCheckIDs {
			if id == known {
				valid = true
			}
		}
		if !valid || seen[id] {
			return invalid("必需能力无效或重复")
		}
		seen[id] = true
	}
	if len(s.References) > 8 {
		return invalid("最多保存 8 个参照渠道")
	}
	seen = map[string]bool{}
	for _, ref := range s.References {
		if !externalToolID.MatchString(ref.ID) || seen[ref.ID] || strings.TrimSpace(ref.Name) == "" || len(ref.Name) > 200 || (ref.Source != "official" && ref.Source != "trusted" && ref.Source != "other") {
			return invalid("参照名称、来源或编号无效")
		}
		if _, err := uuid.Parse(ref.TargetID); err != nil {
			return invalid("请选择有效的参照账号模型")
		}
		seen[ref.ID] = true
	}
	if len(s.ManualReferences) > 50 {
		return invalid("最多保存 50 个手工计数参照")
	}
	seen = map[string]bool{}
	for _, ref := range s.ManualReferences {
		hash, err := hex.DecodeString(ref.RequestHash)
		if !externalToolID.MatchString(ref.ID) || seen[ref.ID] || strings.TrimSpace(ref.Name) == "" || len(ref.Name) > 200 || validateModel(ref.Model) != nil || (ref.Protocol != "chat" && ref.Protocol != "responses" && ref.Protocol != "anthropic") || err != nil || len(hash) != 32 || ref.InputTokens <= 0 || ref.InputTokens > 1000000000 || ref.CapturedAt.IsZero() || ref.CapturedAt.After(time.Now().Add(5*time.Minute)) {
			return invalid("手工参照需填写来源、模型、协议、64 位请求指纹、正整数输入计数和有效日期")
		}
		seen[ref.ID] = true
	}
	return nil
}

func (s *Store) Assurance(ctx context.Context) (AssuranceSettings, error) {
	out := AssuranceSettings{Version: 1, Policy: DefaultScorePolicy("chat"), References: []AuditReference{}}
	out.ManualReferences = []ManualTokenReference{}
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT config,version FROM quality_assurance_settings WHERE id=1`).Scan(&raw, &out.Version)
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, err
}
func (s *Store) SaveAssurance(ctx context.Context, cfg AssuranceSettings) (AssuranceSettings, error) {
	if err := cfg.validate(); err != nil {
		return cfg, err
	}
	raw, err := json.Marshal(struct {
		Policy           ScorePolicy            `json:"policy"`
		References       []AuditReference       `json:"references"`
		ManualReferences []ManualTokenReference `json:"manual_references"`
	}{cfg.Policy, cfg.References, cfg.ManualReferences})
	if err != nil {
		return cfg, err
	}
	err = s.DB.QueryRowContext(ctx, `UPDATE quality_assurance_settings SET config=$1,version=version+1,updated_at=NOW() WHERE id=1 AND version=$2 RETURNING version`, string(raw), cfg.Version).Scan(&cfg.Version)
	if err == sql.ErrNoRows {
		err = conflict("评分设置已在其他窗口修改，请重新载入")
	}
	return cfg, err
}
func (h *Handler) Assurance(c *gin.Context) {
	if !authorized(c) {
		return
	}
	cfg, err := h.Runner.Store.Assurance(c.Request.Context())
	if !respondError(c, err) {
		response.Success(c, cfg)
	}
}
func (h *Handler) SaveAssurance(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var cfg AssuranceSettings
	if !bind(c, &cfg) {
		return
	}
	if respondError(c, cfg.validate()) {
		return
	}
	for _, ref := range cfg.References {
		if _, err := h.Runner.Store.Target(c.Request.Context(), ref.TargetID); respondError(c, err) {
			return
		}
	}
	out, err := h.Runner.Store.SaveAssurance(c.Request.Context(), cfg)
	if !respondError(c, err) {
		response.Success(c, out)
	}
}

// Evidence is an append-only administrative attestation, never an inference
// from a shared wallet balance. Later corrections preserve the full history.
type AssuranceEvidence struct {
	CreatedBy    int64     `json:"created_by"`
	TargetID     string    `json:"target_id,omitempty"`
	ID           string    `json:"id"`
	RunID        string    `json:"run_id"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	Note         string    `json:"note"`
	Receipt      string    `json:"receipt"`
	Currency     string    `json:"currency"`
	Expected     float64   `json:"expected"`
	Actual       float64   `json:"actual"`
	Attributable bool      `json:"attributable"`
	CreatedAt    time.Time `json:"created_at"`
}

func (e AssuranceEvidence) validate() error {
	if _, err := uuid.Parse(e.RunID); err != nil {
		return invalid("检测记录编号无效")
	}
	if (e.Kind != "billing" && e.Kind != "identity" && e.Kind != "privacy") || (e.Status != "suspected" && e.Status != "confirmed" && e.Status != "cleared") || strings.TrimSpace(e.Note) == "" || len(e.Note) > 4000 || strings.TrimSpace(e.Receipt) == "" || len(e.Receipt) > 500 {
		return invalid("请填写问题类别、结论、依据和账单/证据编号")
	}
	if e.Kind == "billing" && (len(e.Currency) != 3 || e.Currency != strings.ToUpper(e.Currency) || strings.Trim(e.Currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" || !e.Attributable || e.Expected <= 0 || e.Actual < 0 || e.Expected > 1e9 || e.Actual > 1e9 || math.IsNaN(e.Expected) || math.IsNaN(e.Actual) || math.IsInf(e.Expected, 0) || math.IsInf(e.Actual, 0)) {
		return invalid("对账需填写同币种金额并确认费用可归属到本次检测；不能使用有并发消费的钱包差额")
	}
	return nil
}
func (h *Handler) SaveAssuranceEvidence(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var e AssuranceEvidence
	if !bind(c, &e) {
		return
	}
	if respondError(c, e.validate()) {
		return
	}
	job, err := scanRun(h.Runner.Store.DB.QueryRowContext(c.Request.Context(), `SELECT `+runColumns+` FROM quality_monitor_runs WHERE id=$1`, e.RunID))
	if respondError(c, err) {
		return
	}
	if job.Status != "finished" || job.Result == nil || job.Result.Requests == 0 {
		response.BadRequest(c, "仅能为已实际请求的检测追加证据")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Forbidden(c, "需要有效的管理员身份")
		return
	}
	e.CreatedBy = subject.UserID
	e.ID = uuid.NewString()
	e.TargetID = job.TargetID
	e.CreatedAt = time.Now()
	raw, _ := json.Marshal(e)
	_, err = h.Runner.Store.DB.ExecContext(c.Request.Context(), `INSERT INTO quality_assurance_evidence(id,run_id,kind,status,detail,created_at) VALUES($1,$2,$3,$4,$5,$6)`, e.ID, e.RunID, e.Kind, e.Status, string(raw), e.CreatedAt)
	if !respondError(c, err) {
		response.Success(c, e)
	}
}
func (s *Store) AssuranceEvidence(ctx context.Context, since time.Time) ([]AssuranceEvidence, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT ON (run_id,kind) detail FROM quality_assurance_evidence WHERE created_at >= $1 ORDER BY run_id,kind,created_at DESC,id DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssuranceEvidence{}
	for rows.Next() {
		var raw []byte
		var e AssuranceEvidence
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
