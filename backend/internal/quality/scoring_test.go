package quality

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func scoredRuns(n int, now time.Time) []Run {
	out := []Run{}
	pass := true
	for i := 0; i < n; i++ {
		ms := int64(1000)
		items := []Sample{}
		for j := 0; j < 4; j++ {
			items = append(items, Sample{CheckID: "reasoning", Correct: &pass, Probe: &service.QualityProbeResult{HTTPStatus: 200, Complete: true, FirstTokenMs: &ms}})
		}
		out = append(out, Run{ID: fmtID(i), Status: "finished", Kind: "inspection", CreatedAt: now.Add(-time.Duration(i) * time.Hour), Result: &Result{Items: items, Checks: []InspectionCheck{{ID: "protocol_stream", Verdict: "passed", Requests: 1}, {ID: "tools", Verdict: "passed", Requests: 2}, {ID: "structured", Verdict: "passed", Requests: 1}, {ID: "context", Verdict: "passed", Requests: 1}}}})
	}
	return out
}
func fmtID(i int) string { return string(rune('a' + i)) }

func TestQualityScoringEvidenceAndGates(t *testing.T) {
	now := time.Now()
	target := Target{ID: "target", AccountID: 1, Model: "test"}
	p := DefaultScorePolicy("chat")
	row := assessTarget(target, scoredRuns(1, now), nil, p, now)
	require.Equal(t, "insufficient", row.State)
	require.Nil(t, row.Rank)
	require.Nil(t, row.Dimensions[4].Value, "missing usage evidence is never perfect")
	require.Nil(t, row.Dimensions[5].Value)
	row = assessTarget(target, scoredRuns(4, now), nil, p, now)
	require.Equal(t, "usable", row.State)
	require.Equal(t, 80, row.Coverage)
	require.NotNil(t, row.Score)
	require.Greater(t, *conservativeScore(100, 100), *conservativeScore(1, 1))

	jobs := scoredRuns(4, now)
	for i := 0; i < 2; i++ {
		jobs[i].Result.Checks[1].Verdict = "failed"
	}
	row = assessTarget(target, jobs, nil, DefaultScorePolicy("agent"), now)
	require.Equal(t, "blocked", row.State, "fast responses cannot cancel repeated required-tool failures")
	row = assessTarget(target, jobs, nil, DefaultScorePolicy("chat"), now)
	require.Equal(t, "usable", row.State, "optional tool failures are a tolerable limitation for chat, not a universal fraud verdict")

	jobs = scoredRuns(4, now)
	jobs[0].Result.Items[0].Probe.Complete = false
	jobs[0].Result.Items[0].Probe.Reason = "upstream_error"
	jobs[0].Result.Items[1].Probe.Complete = false
	jobs[0].Result.Items[1].Probe.Reason = "upstream_error"
	jobs[0].Result.Items[2].Probe.Complete = false
	jobs[0].Result.Items[2].Probe.Reason = "rate_limited"
	jobs[0].Result.Items[3].Probe.Complete = false
	jobs[0].Result.Items[3].Probe.Reason = "authentication"
	row = assessTarget(target, jobs, nil, p, now)
	require.Equal(t, "blocked", row.State)

	old := scoredRuns(4, now.Add(-100*time.Hour))
	row = assessTarget(target, old, nil, p, now)
	require.Equal(t, "stale", row.State)
	require.Equal(t, 0, row.Coverage)
	require.Nil(t, row.Score)
	e := AssuranceEvidence{TargetID: target.ID, RunID: "old-report", Kind: "privacy", Status: "confirmed", Note: "confirmed by admin"}
	row = assessTarget(target, old, []AssuranceEvidence{e}, p, now)
	require.Equal(t, "blocked", row.State, "confirmed critical issues do not expire into a healthy score")

	jobs = scoredRuns(4, now)
	for i := range jobs {
		jobs[i].Result.Reason = "configuration_changed"
	}
	row = assessTarget(target, jobs, nil, p, now)
	require.Nil(t, row.Score)
	require.Equal(t, 0, row.Samples)
}

func TestQualityBillingOnlyAttributableEvidence(t *testing.T) {
	now := time.Now()
	target := Target{ID: "target"}
	jobs := scoredRuns(4, now)
	evidence := AssuranceEvidence{RunID: jobs[0].ID, Kind: "billing", Status: "confirmed", Expected: 1, Actual: 2, Attributable: true}
	row := assessTarget(target, jobs, []AssuranceEvidence{evidence}, DefaultScorePolicy("chat"), now)
	require.Equal(t, "blocked", row.State)
	require.InDelta(t, 50, *row.Dimensions[5].Value, .001)
	evidence.Status = "suspected"
	row = assessTarget(target, jobs, []AssuranceEvidence{evidence}, DefaultScorePolicy("chat"), now)
	require.Nil(t, row.Dimensions[5].Value, "unconfirmed costs cannot enter the score")
	require.Equal(t, "review", row.State)
}

func TestQualityAuditComparisonBoundaries(t *testing.T) {
	primary := AuditSample{Lane: "primary", Probe: &service.QualityProbeResult{Complete: true, ControlledInput: true, RequestHash: "same", Protocol: "chat", InputTokens: 1500, UsageReported: true}}
	ref := AuditSample{Lane: "ref", Probe: &service.QualityProbeResult{Complete: true, ControlledInput: true, RequestHash: "same", Protocol: "chat", InputTokens: 1000, UsageReported: true}}
	c := compareAudit(primary, ref, 15)
	require.Equal(t, "difference", c.Status)
	require.InDelta(t, 50, *c.ExtraPercent, .01)
	require.InDelta(t, 100./3, *c.ReportedSharePercent, .01)
	ref.Probe.RequestHash = "other"
	require.Equal(t, "incomparable", compareAudit(primary, ref, 15).Status)
	ref.Probe.RequestHash = "same"
	ref.Probe.ControlledInput = false
	require.Equal(t, "incomparable", compareAudit(primary, ref, 15).Status)
	ref.Probe.ControlledInput = true
	ref.Probe.UsageReported = false
	require.Equal(t, "usage_missing", compareAudit(primary, ref, 15).Status)
	ref.Probe.UsageReported = true
	ref.Probe.InputTokens = 2000
	c = compareAudit(primary, ref, 15)
	require.Negative(t, *c.ExtraPercent, "retain negative usage deltas")
	ref.Probe.InputTokens = 0
	require.Nil(t, compareAudit(primary, ref, 15).ExtraPercent)

	primary.Probe.Protocol = "anthropic"
	ref.Probe.Protocol = "anthropic"
	primary.Probe.InputTokens = 100
	primary.Probe.CacheReadTokens = 1400
	ref.Probe.InputTokens = 300
	ref.Probe.CacheReadTokens = 1200
	c = compareAudit(primary, ref, 15)
	require.Equal(t, "within_tolerance", c.Status)
	require.Zero(t, c.Difference, "Claude input components are disjoint")
}
func TestQualityAuditPlanAndValidation(t *testing.T) {
	for _, check := range []string{"usage_audit", "stability_audit"} {
		for i := 0; i < inspectionCount(check, "quick"); i++ {
			prompt, expected := auditTask("stable", check, i)
			p2, e2 := auditTask("stable", check, i)
			require.Equal(t, prompt, p2)
			require.Equal(t, expected, e2)
			require.NotEmpty(t, expected)
		}
	}
	for _, i := range []int{0, 2, 4} {
		a, b := auditTask("stable", "usage_audit", i)
		c, d := auditTask("stable", "usage_audit", i+1)
		require.Equal(t, a, c)
		require.Equal(t, b, d)
	}
	o := InspectionOptions{Mode: "quick", Checks: []string{"usage_audit"}}
	require.NoError(t, o.validate())
	require.NotNil(t, o.Audit)
	require.Equal(t, 6, o.count("usage_audit"))
	o.Audit.ReferenceIDs = []string{"same", "same"}
	require.Error(t, o.validate())
	cfg := AssuranceSettings{Version: 1, Policy: DefaultScorePolicy("chat")}
	require.NoError(t, cfg.validate())
	cfg.Policy.Weights["availability"] = 26
	require.Error(t, cfg.validate())
	e := AssuranceEvidence{RunID: "12345678-1234-4234-8234-123456789012", Kind: "billing", Status: "confirmed", Note: "checked", Receipt: "receipt", Currency: "USD", Expected: 1, Actual: 1.2}
	require.Error(t, e.validate())
	e.Attributable = true
	require.NoError(t, e.validate())
	e.Actual = math.Inf(1)
	require.Error(t, e.validate())
}

func TestQualityScoringSpanNativeUnsupportedAndDrift(t *testing.T) {
	now := time.Now()
	p := DefaultScorePolicy("agent")
	target := Target{ID: "t"}
	jobs := scoredRuns(4, now)
	for i := range jobs {
		jobs[i].CreatedAt = now
		jobs[i].Result.Checks[1] = InspectionCheck{ID: "tools", Verdict: "unsupported", Reason: "native_protocol", Requests: 0}
	}
	row := assessTarget(target, jobs, nil, p, now)
	require.Equal(t, "insufficient", row.State)
	for _, issue := range row.Issues {
		require.NotEqual(t, "block", issue.Severity, "an unexpressible native probe is not an upstream failure")
	}
	jobs = scoredRuns(4, now)
	for i := range jobs {
		jobs[i].CreatedAt = now
	}
	row = assessTarget(target, jobs, nil, DefaultScorePolicy("chat"), now)
	require.Equal(t, 0, row.SpanMinutes)
	require.Equal(t, "insufficient", row.State, "several immediate runs do not prove temporal stability")
	older := scoredRuns(6, now.Add(-30*time.Hour))
	recent := scoredRuns(6, now)
	for i := range older {
		older[i].Kind = "quality"
	}
	for i := range recent {
		recent[i].Kind = "quality"
		for j := range recent[i].Result.Items {
			v := false
			recent[i].Result.Items[j].Correct = &v
		}
	}
	trend := capabilityTrend(append(older, recent...), now)
	require.NotNil(t, trend)
	require.True(t, trend.Declined)
	compact := compactScoreIssues([]ScoreIssue{{Code: "same", Severity: "review", RunIDs: []string{"a", "b"}}, {Code: "same", Severity: "review", RunIDs: []string{"a", "c"}}})
	require.Len(t, compact, 1)
	require.Equal(t, []string{"a", "b", "c"}, compact[0].RunIDs)
}
func TestQualityAuditFitAndManualReferenceValidation(t *testing.T) {
	comparisons := []AuditComparison{}
	for _, n := range []int64{100, 100, 1000, 1000, 5000, 5000} {
		comparisons = append(comparisons, AuditComparison{Reference: "r", Status: "difference", ReferenceInput: n, Difference: 50 + n/2})
	}
	fits := fitAudit(comparisons)
	require.Len(t, fits, 1)
	require.InDelta(t, 50, fits[0].FixedExtra, .001)
	require.InDelta(t, .5, fits[0].GrowthExtra, .001)
	require.InDelta(t, 0, fits[0].RMSE, .001)
	require.Empty(t, fitAudit(comparisons[:4]), "two input lengths cannot provide a fit")
	cfg := AssuranceSettings{Version: 1, Policy: DefaultScorePolicy("chat"), ManualReferences: []ManualTokenReference{{ID: "m", Name: "Count receipt", Model: "text-model", Protocol: "chat", RequestHash: strings.Repeat("a", 64), InputTokens: 100, CapturedAt: time.Now()}}}
	require.NoError(t, cfg.validate())
	cfg.ManualReferences[0].RequestHash = "wrong"
	require.Error(t, cfg.validate())
	a := &service.Account{Credentials: map[string]any{"api_key": "one"}}
	old := profileHash(a, Target{Model: "text-model"}, DefaultSettings().Profile)
	a.Credentials["api_key"] = "two"
	require.NotEqual(t, old, profileHash(a, Target{Model: "text-model"}, DefaultSettings().Profile), "credential changes invalidate old quality evidence")
}
