//go:build integration

package quality

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type inspectionTestProber struct {
	calls      int
	incomplete bool
	miss       bool
	after      func()
	answers    map[string]string
	prompts    []string
}

func (p *inspectionTestProber) RunQualityProbe(_ context.Context, _ int64, _ string, o service.QualityProbeOptions) (*service.QualityProbeResult, error) {
	p.calls++
	p.prompts = append(p.prompts, o.Prompt)
	ms := int64(50)
	r := &service.QualityProbeResult{Complete: true, WireValid: true, Model: "reported-model", HTTPStatus: 200, DurationMs: 1000, FirstTokenMs: &ms, InputTokens: 5000, OutputTokens: 20, UsageReported: true, CacheReported: true}
	switch {
	case p.answers[o.Prompt] != "":
		r.Text = p.answers[o.Prompt]
	case o.Tools || o.Structured:
		re := regexp.MustCompile(`nonce="([^"]+)".*value=([0-9]+)`)
		parts := re.FindStringSubmatch(o.Prompt)
		if len(parts) == 3 {
			r.Text = `{"nonce":"` + parts[1] + `","value":` + parts[2] + `}`
		}
		if o.Tools {
			r.ToolCalls = []service.QualityToolCall{{ID: "call-1", Name: "channel_probe", Arguments: r.Text}}
			r.Text = ""
		}
	case o.ToolCallID != "":
		r.Text = o.ToolResult
	case o.Prefix != "":
		r.Text = o.Prompt[strings.LastIndex(o.Prompt, ": ")+2:]
		if !p.miss {
			r.CacheReadTokens = 1024
		}
	case strings.Contains(o.Prompt, "target_key="):
		r.Text = regexp.MustCompile(`target_key=([^\n]+)`).FindStringSubmatch(o.Prompt)[1]
	case strings.Contains(o.Prompt, "verification code and nothing else: "):
		r.Text = o.Prompt[strings.LastIndex(o.Prompt, ": ")+2:]
	default:
		r.Text = "1 2 3 4"
	}
	if p.incomplete {
		r.Complete = false
		r.Reason = "truncated"
	}
	if p.after != nil {
		p.after()
	}
	return r, nil
}

func TestQualityPostgresCustomInspectionSnapshotsAndGrading(t *testing.T) {
	db := qualityTestDB(t)
	ctx := context.Background()
	a := &service.Account{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "private-key"}}
	prober := &inspectionTestProber{answers: map[string]string{"exact prompt": "Wrong", "keyword prompt": "beta and alpha", "open prompt": "An essay", "last prompt": "Tail"}}
	runner := NewRunner(db, &testAccounts{a}, prober, &testSlots{acquired: true}, "UTC")
	cfg, err := runner.Store.Settings(ctx)
	require.NoError(t, err)
	cfg.AutoEnabled = false
	cfg.InspectionQuestions = []InspectionQuestion{
		{Name: "Exact", Prompt: "exact prompt", Match: "exact", Expected: "Secret expected answer"},
		{Name: "Keywords", Prompt: "keyword prompt", Match: "contains", Expected: "alpha\nbeta"},
		{Name: "Open", Prompt: "open prompt", Match: "manual", Expected: "Private review notes"},
		{Name: "Last", Prompt: "last prompt", Match: "exact", Expected: "Tail"},
	}
	cfg, err = runner.Store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	loaded, err := runner.Store.Settings(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.InspectionQuestions, loaded.InspectionQuestions)
	target, err := runner.Store.SaveTarget(ctx, Target{AccountID: 1, Model: "text-model", Enabled: false}, cfg)
	require.NoError(t, err)
	options := InspectionOptions{Mode: "quick", Checks: []string{"custom"}, Questions: cfg.InspectionQuestions}
	queued, err := runner.Store.QueueInspection(ctx, target.ID, options)
	require.NoError(t, err)
	// The queued task retains the reviewed questions when saved templates change.
	cfg.InspectionQuestions = nil
	cfg, err = runner.Store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	job, err := runner.Store.Claim(ctx)
	require.NoError(t, err)
	require.Equal(t, queued.Inspection, job.Inspection)
	require.NoError(t, runner.execute(ctx, *job, cfg))
	reports, err := runner.Store.historyKind(ctx, target.ID, 60, "inspection")
	require.NoError(t, err)
	result := reports[0].Result
	require.Equal(t, "suspected", reports[0].Verdict)
	require.Equal(t, "failed", result.Checks[0].Verdict)
	require.Equal(t, 4, result.Requests)
	require.Equal(t, 4, result.Planned)
	require.False(t, *result.Items[0].Correct)
	require.True(t, *result.Items[1].Correct)
	require.Nil(t, result.Items[2].Correct)
	require.Equal(t, "manual_review", result.Items[2].GradeReason)
	require.True(t, *result.Items[3].Correct)
	require.Equal(t, "Last", result.Items[3].Name)
	require.Equal(t, "exact", result.Items[3].Match)
	require.Equal(t, []string{"exact prompt", "keyword prompt", "open prompt", "last prompt"}, prober.prompts)
	for _, prompt := range prober.prompts {
		require.NotContains(t, prompt, "Secret expected answer")
		require.NotContains(t, prompt, "Private review notes")
	}
	_, err = runner.Store.QueueInspection(ctx, target.ID, InspectionOptions{Mode: "quick", Checks: []string{"custom"}, Questions: options.Questions[2:3]})
	require.NoError(t, err)
	job, err = runner.Store.Claim(ctx)
	require.NoError(t, err)
	require.NoError(t, runner.execute(ctx, *job, cfg))
	reports, err = runner.Store.historyKind(ctx, target.ID, 60, "inspection")
	require.NoError(t, err)
	require.Equal(t, "unknown", reports[0].Verdict)
	require.Equal(t, "manual_review", reports[0].Result.Checks[0].Reason)
}

func TestQualityPostgresInspectionQueueEvidenceAndIsolation(t *testing.T) {
	db := qualityTestDB(t)
	ctx := context.Background()
	a := &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "private-key"}}
	prober := &inspectionTestProber{}
	runner := NewRunner(db, &testAccounts{a}, prober, &testSlots{acquired: true}, "UTC")
	cfg, err := runner.Store.Settings(ctx)
	require.NoError(t, err)
	target, err := runner.Store.SaveTarget(ctx, Target{AccountID: 1, Model: "text-model", Enabled: false}, cfg)
	require.NoError(t, err)
	target, err = runner.Store.Target(ctx, target.ID)
	require.NoError(t, err)
	execute := func(checks []string) Run {
		queued, err := runner.Store.QueueInspection(ctx, target.ID, InspectionOptions{Mode: "full", Checks: checks})
		require.NoError(t, err)
		require.Equal(t, "inspection", queued.Kind)
		require.True(t, queued.Manual)
		job, err := runner.Store.Claim(ctx)
		require.NoError(t, err)
		require.NotNil(t, job)
		require.NoError(t, runner.execute(ctx, *job, cfg))
		reports, err := runner.Store.historyKind(ctx, target.ID, 60, "inspection")
		require.NoError(t, err)
		require.Equal(t, queued.ID, reports[0].ID)
		return reports[0]
	}
	report := execute([]string{"protocol_stream", "protocol_nonstream", "structured", "tools", "speed", "cache", "context"})
	require.Equal(t, "passed", report.Verdict)
	require.Len(t, report.Result.Checks, 7)
	require.Equal(t, 14, report.Result.Requests)
	for _, c := range report.Result.Checks {
		require.Equal(t, "passed", c.Verdict, c.ID)
		require.NotEmpty(t, c.SampleIDs)
	}
	require.NotNil(t, report.Result.Checks[5].CacheRatio)
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private-key")
	budget, err := runner.Store.Budget(ctx, cfg, report.CreatedAt, runner.Location)
	require.NoError(t, err)
	require.Equal(t, 14, budget.ManualRequests)
	require.Zero(t, budget.Requests)
	after, err := runner.Store.Target(ctx, target.ID)
	require.NoError(t, err)
	require.False(t, after.Enabled)
	require.Equal(t, target.NextQualityAt, after.NextQualityAt)
	var logs int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM usage_logs`).Scan(&logs))
	require.Zero(t, logs)
	prober.miss = true
	report = execute([]string{"cache"})
	require.Equal(t, "not_observed", report.Result.Checks[0].Verdict)
	require.Equal(t, "unknown", report.Verdict)
	prober.incomplete = true
	report = execute([]string{"protocol_stream"})
	require.Equal(t, "uncompleted", report.Result.Checks[0].Verdict)
	require.Equal(t, "unknown", report.Verdict)
	prober.incomplete = false
	prober.after = func() { a.Credentials["api_key"] = "changed" }
	report = execute([]string{"protocol_stream", "speed"})
	require.Equal(t, "configuration_changed", report.Result.Reason)
	require.Equal(t, 1, report.Result.Requests)
	require.Equal(t, "uncompleted", report.Result.Checks[1].Verdict)
}
