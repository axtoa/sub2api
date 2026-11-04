package quality

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ScoreDimension struct {
	ID      string   `json:"id"`
	Value   *float64 `json:"value"`
	Samples int      `json:"samples"`
	Passed  int      `json:"passed"`
	P95MS   *int64   `json:"p95_ms"`
}
type ScoreIssue struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"` // block / review / tolerated / unknown
	Check    string   `json:"check,omitempty"`
	RunIDs   []string `json:"run_ids"`
}
type QualityScore struct {
	ComparisonSet []string            `json:"comparison_set"`
	SpanMinutes   int                 `json:"span_minutes"`
	Trend         *ScoreTrend         `json:"trend,omitempty"`
	TargetID      string              `json:"target_id"`
	AccountID     int64               `json:"account_id"`
	AccountName   string              `json:"account_name"`
	Platform      string              `json:"platform"`
	Model         string              `json:"model"`
	Cohort        string              `json:"cohort"`
	Score         *float64            `json:"score"`
	Coverage      int                 `json:"coverage"`
	Samples       int                 `json:"samples"`
	Runs          int                 `json:"runs"`
	Excluded      int                 `json:"excluded"`
	Truncated     bool                `json:"truncated"`
	Rank          *int                `json:"rank"`
	State         string              `json:"state"`
	LastAt        *time.Time          `json:"last_at"`
	Dimensions    []ScoreDimension    `json:"dimensions"`
	Issues        []ScoreIssue        `json:"issues"`
	Evidence      []AssuranceEvidence `json:"evidence"`
	RunIDs        []string            `json:"run_ids"`
}

type ScoreTrend struct {
	RecentCorrect   int  `json:"recent_correct"`
	RecentSamples   int  `json:"recent_samples"`
	PreviousCorrect int  `json:"previous_correct"`
	PreviousSamples int  `json:"previous_samples"`
	Declined        bool `json:"declined"`
}

func capabilitySample(job Run, s Sample) bool {
	return job.Kind == "quality" || s.CheckID == "reasoning" || s.CheckID == "vision" || s.CheckID == "context" || s.CheckID == "custom" || s.CheckID == "stability_audit"
}

func capabilityTrend(runs []Run, now time.Time) *ScoreTrend {
	t := &ScoreTrend{}
	for _, job := range runs {
		if job.Kind != "quality" || job.Result == nil || job.Status != "finished" || job.Result.Reason == "configuration_changed" || job.Result.Reason == "interrupted" {
			continue
		}
		at := job.CreatedAt
		if job.FinishedAt != nil {
			at = *job.FinishedAt
		}
		for _, s := range job.Result.Items {
			if s.Probe == nil || !s.Probe.Complete || s.Correct == nil || !capabilitySample(job, s) {
				continue
			}
			if now.Sub(at) <= 24*time.Hour {
				t.RecentSamples++
				if *s.Correct {
					t.RecentCorrect++
				}
			} else {
				t.PreviousSamples++
				if *s.Correct {
					t.PreviousCorrect++
				}
			}
		}
	}
	if t.RecentSamples < 20 || t.PreviousSamples < 20 {
		return nil
	}
	previousLower := *conservativeScore(t.PreviousCorrect, t.PreviousSamples)
	recentUpper := 100 - *conservativeScore(t.RecentSamples-t.RecentCorrect, t.RecentSamples)
	t.Declined = recentUpper < previousLower
	return t
}

type ScoreBoard struct {
	Version     string         `json:"version"`
	GeneratedAt time.Time      `json:"generated_at"`
	Days        int            `json:"days"`
	Policy      ScorePolicy    `json:"policy"`
	Items       []QualityScore `json:"items"`
}

// Wilson's 95% lower confidence bound prevents one lucky pass from earning
// the same reliability score as a long, clean record. This is a measured score,
// never a probability that the account is a genuine particular model.
func conservativeScore(pass, n int) *float64 {
	if n == 0 {
		return nil
	}
	z := 1.96
	p := float64(pass) / float64(n)
	f := float64(n)
	v := 100 * (p + z*z/(2*f) - z*math.Sqrt((p*(1-p)+z*z/(4*f))/f)) / (1 + z*z/f)
	v = math.Round(math.Max(0, v)*10) / 10
	return &v
}
func scoreCohort(t Target, p Profile) string {
	raw, _ := json.Marshal([]any{GeneratorVersion, t.Platform, t.Model, p.Questions, p.PassPercent, p.MaxOutputTokens, p.ReasoningEffort, p.CachePrefixKB})
	sum := sha256.Sum256(raw)
	return t.Platform + ":" + t.Model + ":" + strconv.FormatUint(uint64(sum[0])<<24|uint64(sum[1])<<16|uint64(sum[2])<<8|uint64(sum[3]), 16)
}
func probeObserved(p *service.QualityProbeResult) bool {
	if p == nil {
		return false
	}
	switch p.Reason {
	case "unsupported", "capacity_unavailable", "budget_limited", "budget_unavailable", "configuration_changed", "account_paused", "disabled", "interrupted":
		return false
	}
	return p.HTTPStatus != 0 || p.Reason == "timeout" || p.Reason == "transport_error"
}

func assessTarget(t Target, runs []Run, evidence []AssuranceEvidence, p ScorePolicy, now time.Time) QualityScore {
	out := QualityScore{TargetID: t.ID, AccountID: t.AccountID, AccountName: t.AccountName, Platform: t.Platform, Model: t.Model, State: "insufficient", Dimensions: []ScoreDimension{}, Issues: []ScoreIssue{}, Evidence: []AssuranceEvidence{}, RunIDs: []string{}}
	out.Trend = capabilityTrend(runs, now)
	if out.Trend != nil && out.Trend.Declined {
		out.Issues = append(out.Issues, ScoreIssue{Code: "capability_decline", Severity: "review", RunIDs: []string{}})
	}
	dims := map[string]*ScoreDimension{}
	for _, id := range scoreDimensions {
		d := &ScoreDimension{ID: id}
		dims[id] = d
	}
	runSet := map[string]bool{}
	checkRuns := map[string][]string{}
	badChecks := map[string][]string{}
	unsupported := map[string][]string{}
	latencies := []int64{}
	var earliest *time.Time
	failures := 0
	deltas := 0
	for _, job := range runs {
		if job.Status != "finished" || job.Result == nil || job.Result.Reason == "configuration_changed" || job.Result.Reason == "interrupted" {
			out.Excluded++
			continue
		}
		at := job.CreatedAt
		if job.FinishedAt != nil {
			at = *job.FinishedAt
		}
		if now.Sub(at) > time.Duration(p.FreshHours)*time.Hour {
			if len(job.Result.Items) > 0 && (out.LastAt == nil || at.After(*out.LastAt)) {
				out.LastAt = &at
			}
			out.Excluded++
			continue
		}
		observed := 0
		for _, s := range job.Result.Items {
			probe := s.Probe
			if !probeObserved(probe) {
				continue
			}
			observed++
			out.Samples++
			d := dims["availability"]
			d.Samples++
			if probe.Complete {
				d.Passed++
			} else {
				failures++
			}
			if probe.Complete && probe.FirstTokenMs != nil && *probe.FirstTokenMs >= 0 {
				latencies = append(latencies, *probe.FirstTokenMs)
			}
			if len(probe.UsageAnomalies) > 0 {
				out.Issues = append(out.Issues, ScoreIssue{Code: "invalid_usage", Severity: "review", RunIDs: []string{job.ID}})
			}
			if probe.Complete && s.Correct != nil && (job.Kind == "quality" || s.CheckID == "reasoning" || s.CheckID == "vision" || s.CheckID == "context" || s.CheckID == "custom" || s.CheckID == "stability_audit") {
				d = dims["capability"]
				d.Samples++
				if *s.Correct {
					d.Passed++
				}
			}
		}
		if observed == 0 {
			out.Excluded++
			continue
		}
		if earliest == nil || at.Before(*earliest) {
			earliest = &at
		}
		out.Runs++
		runSet[job.ID] = true
		if len(out.RunIDs) < 20 {
			out.RunIDs = append(out.RunIDs, job.ID)
		}
		if out.LastAt == nil || at.After(*out.LastAt) {
			out.LastAt = &at
		}
		for _, c := range job.Result.Checks {
			if c.Requests == 0 {
				// The probe cannot express some native OAuth features. That is
				// missing evidence, not proof that the upstream lacks the feature.
				continue
			}
			if c.Verdict == "passed" || c.Verdict == "failed" || c.Verdict == "unsupported" {
				checkRuns[c.ID] = append(checkRuns[c.ID], job.ID)
			}
			if c.Verdict == "failed" {
				badChecks[c.ID] = append(badChecks[c.ID], job.ID)
			}
			if c.Verdict == "unsupported" {
				unsupported[c.ID] = append(unsupported[c.ID], job.ID)
			}
			switch c.ID {
			case "protocol_stream", "protocol_nonstream", "tools", "structured":
				if c.Verdict == "passed" || c.Verdict == "failed" || c.Verdict == "unsupported" {
					d := dims["compatibility"]
					d.Samples++
					if c.Verdict == "passed" {
						d.Passed++
					}
				}
			}
		}
		if job.Result.Audit != nil {
			for _, c := range job.Result.Audit.Comparisons {
				if c.Status != "within_tolerance" && c.Status != "difference" {
					continue
				}
				if c.Reference == "repeat" {
					continue
				}
				d := dims["integrity"]
				d.Samples++
				extra := float64(0)
				if c.ReferenceInput > 0 {
					extra = math.Abs(float64(c.Difference) * 100 / float64(c.ReferenceInput))
				}
				if math.Abs(float64(c.Difference)) <= 32 || extra <= float64(p.UsageTolerancePercent) {
					d.Passed++
				} else {
					deltas++
					out.Issues = append(out.Issues, ScoreIssue{Code: "usage_difference", Severity: "review", RunIDs: []string{job.ID}})
				}
			}
			for _, c := range job.Result.Audit.Comparisons {
				if c.Reference == "repeat" && c.Status == "difference" {
					out.Issues = append(out.Issues, ScoreIssue{Code: "repeat_usage_difference", Severity: "review", RunIDs: []string{job.ID}})
				}
			}
		}
	}
	for _, id := range scoreDimensions {
		d := dims[id]
		d.Value = conservativeScore(d.Passed, d.Samples)
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p95 := latencies[int(math.Ceil(float64(len(latencies))*.95))-1]
		d := dims["latency"]
		d.P95MS = &p95
		d.Samples = len(latencies)
		v := 100 * math.Min(1, float64(p.SlowMS)/float64(max(p95, 1)))
		v = math.Round(v*10) / 10
		d.Value = &v
		if p95 > p.SlowMS {
			out.Issues = append(out.Issues, ScoreIssue{Code: "slow", Severity: "tolerated", RunIDs: []string{}})
		}
	}
	billingTotal := float64(0)
	for _, e := range evidence {
		if !runSet[e.RunID] && e.TargetID != t.ID {
			continue
		}
		out.Evidence = append(out.Evidence, e)
		if e.Status == "confirmed" && (e.Kind == "identity" || e.Kind == "privacy" || (e.Kind == "billing" && e.Actual > e.Expected*(1+float64(p.BillingTolerancePercent)/100))) {
			out.Issues = append(out.Issues, ScoreIssue{Code: e.Kind, Severity: "block", RunIDs: []string{e.RunID}})
		}
		if e.Status == "suspected" {
			out.Issues = append(out.Issues, ScoreIssue{Code: e.Kind, Severity: "review", RunIDs: []string{e.RunID}})
		}
		if runSet[e.RunID] && e.Kind == "billing" && e.Attributable && e.Expected > 0 && (e.Status == "confirmed" || e.Status == "cleared") {
			d := dims["billing"]
			d.Samples++
			v := 100.
			if e.Actual > e.Expected {
				v = 100 * e.Expected / e.Actual
			}
			billingTotal += v
		}
	}
	if dims["billing"].Samples > 0 {
		v := math.Round(billingTotal/float64(dims["billing"].Samples)*10) / 10
		dims["billing"].Value = &v
	}
	if out.Samples >= p.MinSamples && failures >= 3 && failures*100 >= out.Samples*p.FailurePercent {
		out.Issues = append(out.Issues, ScoreIssue{Code: "repeated_unavailable", Severity: "block", RunIDs: out.RunIDs})
	}
	requiredOK := true
	for _, id := range p.RequiredChecks {
		if len(badChecks[id]) >= 2 || len(unsupported[id]) > 0 {
			out.Issues = append(out.Issues, ScoreIssue{Code: "required_failed", Check: id, Severity: "block", RunIDs: append(badChecks[id], unsupported[id]...)})
		}
		if len(checkRuns[id]) < 2 {
			requiredOK = false
			out.Issues = append(out.Issues, ScoreIssue{Code: "required_missing", Check: id, Severity: "unknown", RunIDs: []string{}})
		}
	}
	// A check failure is useful even when that capability is optional for the
	// chosen use. Unknown, skipped and native-unsupported checks never pass.
	for id, ids := range badChecks {
		required := false
		for _, r := range p.RequiredChecks {
			if r == id {
				required = true
			}
		}
		if !required {
			out.Issues = append(out.Issues, ScoreIssue{Code: "optional_failed", Check: id, Severity: "tolerated", RunIDs: ids})
		}
	}
	minimums := map[string]int{"availability": p.MinSamples, "capability": 4, "compatibility": 3, "latency": 3, "integrity": 6, "billing": 1}
	total := 0.
	measured := 0
	for _, id := range scoreDimensions {
		d := dims[id]
		out.Dimensions = append(out.Dimensions, *d)
		if d.Value != nil && d.Samples >= minimums[id] {
			out.ComparisonSet = append(out.ComparisonSet, id)
			out.Coverage += p.Weights[id]
			total += *d.Value * float64(p.Weights[id])
			measured += p.Weights[id]
		}
	}
	if measured > 0 {
		v := math.Round(total/float64(measured)*10) / 10
		out.Score = &v
	}
	if earliest != nil && out.LastAt != nil {
		out.SpanMinutes = int(out.LastAt.Sub(*earliest).Minutes())
	}
	if out.LastAt != nil && now.Sub(*out.LastAt) > time.Duration(p.FreshHours)*time.Hour {
		out.State = "stale"
	} else if out.Samples >= p.MinSamples && out.Runs >= p.MinRuns && out.SpanMinutes >= p.MinSpanMinutes && out.Coverage >= p.MinCoverage && requiredOK {
		out.State = "usable"
		if out.Score != nil && *out.Score >= 85 {
			out.State = "strong"
		} else if out.Score != nil && *out.Score < 65 {
			out.State = "weak"
		}
	}
	if deltas > 0 && out.State != "insufficient" && out.State != "stale" {
		out.State = "review"
	}
	for _, i := range out.Issues {
		if i.Severity == "review" && out.State != "insufficient" && out.State != "stale" {
			out.State = "review"
		}
	}
	for _, i := range out.Issues {
		if i.Severity == "block" {
			out.State = "blocked"
			break
		}
	}
	out.Issues = compactScoreIssues(out.Issues)
	return out
}

func compactScoreIssues(issues []ScoreIssue) []ScoreIssue {
	byKey := map[string]*ScoreIssue{}
	for _, issue := range issues {
		key := issue.Severity + "|" + issue.Code + "|" + issue.Check
		row := byKey[key]
		if row == nil {
			copy := issue
			copy.RunIDs = []string{}
			row = &copy
			byKey[key] = row
		}
		for _, id := range issue.RunIDs {
			found := false
			for _, old := range row.RunIDs {
				if old == id {
					found = true
					break
				}
			}
			if !found && len(row.RunIDs) < 20 {
				row.RunIDs = append(row.RunIDs, id)
			}
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ScoreIssue, 0, len(keys))
	for _, key := range keys {
		out = append(out, *byKey[key])
	}
	return out
}

// One bounded bulk query, no N+1 histories and no silent short-window claim.
// Payload prompts are stripped in SQL; scoring retains only the needed evidence.
func (s *Store) scoreRuns(ctx context.Context, since time.Time) (map[string][]Run, error) {
	const capPerTarget = 1000
	columns := strings.Replace(runColumns, "result,", `jsonb_build_object('reason',result->'reason','requests',result->'requests','checks',result->'checks','audit',jsonb_build_object('comparisons',COALESCE(result->'audit'->'comparisons','[]'::jsonb)),'items',COALESCE((SELECT jsonb_agg(jsonb_build_object('check_id',s->'check_id','correct',s->'correct','probe',(s->'probe')-'text'-'tool_calls')) FROM jsonb_array_elements(COALESCE(NULLIF(result->'items','null'::jsonb),'[]'::jsonb)) s),'[]'::jsonb)) AS result,`, 1)
	// The per-target cap is explicit in the response and prevents huge history
	// reads from monopolising the admin endpoint.
	rows, err := s.DB.QueryContext(ctx, "SELECT "+columns+" FROM (SELECT *,row_number() OVER(PARTITION BY target_id ORDER BY finished_at DESC,id DESC) AS rn FROM quality_monitor_runs WHERE status='finished' AND finished_at >= $1) r WHERE rn <= $2 ORDER BY finished_at DESC,id DESC", since, capPerTarget+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string][]Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out[r.TargetID] = append(out[r.TargetID], r)
	}
	return out, rows.Err()
}
func (h *Handler) Scores(c *gin.Context) {
	if !authorized(c) {
		return
	}
	days := 7
	if c.Query("days") != "" {
		v, e := strconv.Atoi(c.Query("days"))
		if e != nil || (v != 1 && v != 7 && v != 30) {
			response.BadRequest(c, "请选择 1、7 或 30 天")
			return
		}
		days = v
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	assurance, err := h.Runner.Store.Assurance(ctx)
	if respondError(c, err) {
		return
	}
	policy := assurance.Policy
	if scenario := c.Query("scenario"); scenario != "" && scenario != policy.Scenario {
		if scenario != "chat" && scenario != "agent" && scenario != "cost" {
			response.BadRequest(c, "用途无效")
			return
		}
		policy = DefaultScorePolicy(scenario)
	}
	cfg, err := h.Runner.Store.Settings(ctx)
	if respondError(c, err) {
		return
	}
	targets, err := h.Runner.Store.Targets(ctx)
	if respondError(c, err) {
		return
	}
	now := time.Now()
	history, err := h.Runner.Store.scoreRuns(ctx, now.Add(-time.Duration(days)*24*time.Hour))
	if respondError(c, err) {
		return
	}
	evidence, err := h.Runner.Store.AssuranceEvidence(ctx, time.Time{})
	if respondError(c, err) {
		return
	}
	board := ScoreBoard{Version: "assurance-v1", GeneratedAt: now, Days: days, Policy: policy, Items: []QualityScore{}}
	accounts := map[int64]*service.Account{}
	for _, t := range targets {
		a, ok := accounts[t.AccountID]
		if !ok {
			a, err = h.Runner.Accounts.GetByID(ctx, t.AccountID)
			if err != nil {
				a = nil
			}
			accounts[t.AccountID] = a
		}
		valid := []Run{}
		excluded := 0
		truncated := len(history[t.ID]) > 1000
		baseHash := ""
		if a != nil {
			baseHash = profileHash(a, t, t.Effective(cfg), cfg.QuestionBank)
		}
		for i, job := range history[t.ID] {
			if i >= 1000 {
				break
			}
			expected := ""
			if a != nil {
				expected = baseHash
				if job.Kind == "inspection" && job.Inspection != nil {
					expected = inspectionHashFromBase(baseHash, *job.Inspection)
				}
			}
			if expected == "" || job.ProfileHash != expected {
				excluded++
				continue
			}
			valid = append(valid, job)
		}
		score := assessTarget(t, valid, evidence, policy, now)
		score.Cohort = scoreCohort(t, t.Effective(cfg))
		if a == nil {
			score.Issues = append(score.Issues, ScoreIssue{Code: "account_unavailable", Severity: "unknown", RunIDs: []string{}})
		}
		score.Excluded += excluded
		score.Truncated = truncated
		if truncated {
			if score.State != "blocked" {
				score.State = "insufficient"
			}
			score.Issues = append(score.Issues, ScoreIssue{Code: "history_truncated", Severity: "unknown", RunIDs: []string{}})
		}
		board.Items = append(board.Items, score)
	}
	// Each cohort has its own ranks. Unproven or blocked accounts remain visible.
	groups := map[string][]int{}
	for i, row := range board.Items {
		if row.State == "strong" || row.State == "usable" || row.State == "weak" {
			key := row.Cohort + "|" + strings.Join(row.ComparisonSet, ",")
			groups[key] = append(groups[key], i)
		}
	}
	for _, indices := range groups {
		sort.SliceStable(indices, func(i, j int) bool { return *board.Items[indices[i]].Score > *board.Items[indices[j]].Score })
		for rank, index := range indices {
			r := rank + 1
			board.Items[index].Rank = &r
		}
	}
	response.Success(c, board)
}
