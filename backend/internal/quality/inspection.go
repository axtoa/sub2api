package quality

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InspectionOptions struct {
	Mode      string               `json:"mode"`
	Checks    []string             `json:"checks"`
	Questions []InspectionQuestion `json:"questions,omitempty"`
}

type InspectionQuestion struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Match    string `json:"match"` // exact, contains (all nonempty lines), manual
	Expected string `json:"expected"`
}

func validateInspectionQuestions(questions []InspectionQuestion) error {
	if len(questions) > 8 {
		return invalid("自定义检验最多 8 道题")
	}
	total := 0
	for _, q := range questions {
		total += len(q.Name) + len(q.Prompt) + len(q.Expected)
		if strings.TrimSpace(q.Name) == "" || utf8.RuneCountInString(q.Name) > 100 || strings.TrimSpace(q.Prompt) == "" || utf8.RuneCountInString(q.Prompt) > 4000 || utf8.RuneCountInString(q.Expected) > 1000 || total > 24000 {
			return invalid("请填写题目名称和问题，单题问题最多 4000 字、答案最多 1000 字，整组最多 24 KB")
		}
		if q.Match != "exact" && q.Match != "contains" && q.Match != "manual" {
			return invalid("请选择有效的判定方式")
		}
		if q.Match != "manual" && strings.TrimSpace(q.Expected) == "" {
			return invalid("自动判定需要填写预期答案或关键词")
		}
	}
	return nil
}

func gradeInspectionQuestion(q InspectionQuestion, answer string) (*bool, string) {
	if q.Match == "manual" {
		return nil, "manual_review"
	}
	pass := strings.TrimSpace(answer) == strings.TrimSpace(q.Expected)
	if q.Match == "contains" {
		pass = true
		for _, keyword := range strings.Split(q.Expected, "\n") {
			if keyword = strings.TrimSpace(keyword); keyword != "" && !strings.Contains(answer, keyword) {
				pass = false
			}
		}
	}
	return &pass, ""
}

type InspectionCheck struct {
	ID                 string   `json:"id"`
	Verdict            string   `json:"verdict"`
	Reason             string   `json:"reason,omitempty"`
	Requests           int      `json:"requests"`
	SampleIDs          []string `json:"sample_ids"`
	MedianFirstTokenMs *int64   `json:"median_first_token_ms"`
	TokensPerSecond    *float64 `json:"tokens_per_second"`
	CacheRatio         *float64 `json:"cache_ratio"`
}

var inspectionCheckIDs = []string{"protocol_stream", "protocol_nonstream", "tools", "structured", "reasoning", "speed", "cache", "context", "vision", "custom"}

func (o *InspectionOptions) validate() error {
	if o.Mode != "quick" && o.Mode != "full" {
		return invalid("请选择快速或完整检验")
	}
	if len(o.Checks) == 0 || len(o.Checks) > len(inspectionCheckIDs) {
		return invalid("请选择检验项目")
	}
	seen := map[string]bool{}
	for _, id := range o.Checks {
		valid := false
		for _, known := range inspectionCheckIDs {
			if id == known {
				valid = true
			}
		}
		if !valid || seen[id] {
			return invalid("检验项目无效或重复")
		}
		seen[id] = true
	}
	if err := validateInspectionQuestions(o.Questions); err != nil {
		return err
	}
	if seen["custom"] != (len(o.Questions) > 0) {
		return invalid("自定义检验需要选择项目并填写至少一道题")
	}
	// Stable ordering makes request counts and reports reproducible.
	o.Checks = nil
	for _, id := range inspectionCheckIDs {
		if seen[id] {
			o.Checks = append(o.Checks, id)
		}
	}
	return nil
}

func (o InspectionOptions) count(id string) int {
	if id == "custom" {
		return len(o.Questions)
	}
	return inspectionCount(id, o.Mode)
}

func inspectionCount(id, mode string) int {
	switch id {
	case "tools":
		return 2
	case "cache":
		return 3
	case "reasoning":
		if mode == "full" {
			return 4
		}
		return 2
	case "speed":
		if mode == "full" {
			return 5
		}
		return 2
	default:
		return 1
	}
}

func (s *Store) QueueInspection(ctx context.Context, id string, o InspectionOptions) (Run, error) {
	if err := o.validate(); err != nil {
		return Run{}, err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return Run{}, err
	}
	return scanRun(s.DB.QueryRowContext(ctx, `INSERT INTO quality_monitor_runs(id,target_id,kind,status,manual,inspection_options)
 VALUES($1,$2,'inspection','queued',TRUE,$3) RETURNING `+runColumns, uuid.NewString(), id, string(raw)))
}

func (h *Handler) Inspect(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var req struct {
		TargetID string            `json:"target_id"`
		Options  InspectionOptions `json:"options"`
	}
	if !bind(c, &req) {
		return
	}
	if _, err := uuid.Parse(req.TargetID); err != nil {
		response.BadRequest(c, "账号模型编号无效")
		return
	}
	if respondError(c, req.Options.validate()) {
		return
	}
	target, err := h.Runner.Store.Target(c.Request.Context(), req.TargetID)
	if respondError(c, err) {
		return
	}
	account, err := h.Runner.Accounts.GetByID(c.Request.Context(), target.AccountID)
	if err != nil || !service.QualityProbeSupported(account, target.Model) {
		response.BadRequest(c, "当前账号模型不支持检验")
		return
	}
	job, err := h.Runner.Store.QueueInspection(c.Request.Context(), target.ID, req.Options)
	if !respondError(c, err) {
		h.Runner.Wake()
		response.Accepted(c, job)
	}
}

func (r *Runner) executeInspection(ctx context.Context, job *Run, t Target, cfg Settings, p Profile, a *service.Account) error {
	o := *job.Inspection
	if err := o.validate(); err != nil {
		job.Result.Reason = "unsupported"
		return nil
	}
	job.Result.Checks = []InspectionCheck{}
	for _, id := range o.Checks {
		job.Result.Planned += o.count(id)
		job.Result.Checks = append(job.Result.Checks, InspectionCheck{ID: id, Verdict: "uncompleted", SampleIDs: []string{}})
	}
	// A fresh account/profile check surrounds every paid request, including the
	// last one. Credentials are compared in memory and never stored in reports.
	originalIdentity := inspectionIdentity(a)
	ensure := func() (*service.Account, error) {
		current, err := r.Store.Target(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		settings, err := r.Store.Settings(ctx)
		if err != nil {
			return nil, err
		}
		account, err := r.Accounts.GetByID(ctx, t.AccountID)
		if err != nil || !service.QualityProbeSupported(account, t.Model) || inspectionIdentity(account) != originalIdentity || configurationChanged(current, settings, t, cfg, p, *job) || inspectionProfileHash(account, t, p, cfg.QuestionBank, o) != job.ProfileHash {
			return nil, errors.New("configuration_changed")
		}
		if ctx.Err() != nil {
			return nil, errors.New("timeout")
		}
		return account, nil
	}
	fatal := ""
	for ci := range job.Result.Checks {
		check := &job.Result.Checks[ci]
		if fatal != "" {
			check.Reason = fatal
			continue
		}
		// Codex OAuth uses a native streaming-only endpoint. Do not claim a JSON
		// or forced-tool test ran when that transport cannot express it.
		if a.Platform == service.PlatformOpenAI && a.Type == service.AccountTypeOAuth && (check.ID == "protocol_nonstream" || check.ID == "tools" || check.ID == "structured") {
			check.Verdict = "unsupported"
			check.Reason = "native_protocol"
			continue
		}
		check.Verdict = "passed"
		prefix := ""
		if check.ID == "cache" {
			prefix = cachePrefix(job.ID, p.CachePrefixKB)
			job.Result.CachePrefix = prefix
		}
		nonce := uuid.NewString()
		value := int(nonce[0]) + 37
		var tool service.QualityToolCall
		receipt := uuid.NewString()
		var latencies []int64
		var outputTokens, generationMs int64
		var cacheInput, cacheRead int64
		cacheKnown := true
		for i := 0; i < o.count(check.ID); i++ {
			current, err := ensure()
			if err != nil {
				fatal = "configuration_changed"
				check.Verdict = "uncompleted"
				check.Reason = fatal
				job.Result.Reason = fatal
				break
			}
			if err = r.Store.Renew(ctx, *job, p, job.ProfileHash); err != nil {
				return err
			}
			opts := service.QualityProbeOptions{Inspection: true, MaxOutputTokens: p.MaxOutputTokens, ReasoningEffort: p.ReasoningEffort}
			expected := ""
			q := Question{ID: uuid.NewString(), Family: check.ID}
			switch check.ID {
			case "custom":
				question := o.Questions[i]
				opts.Prompt, expected = question.Prompt, question.Expected
			case "protocol_stream", "protocol_nonstream":
				expected = nonce
				opts.Prompt = "Reply with exactly this verification code and nothing else: " + nonce
				opts.NonStream = check.ID == "protocol_nonstream"
			case "structured":
				opts.NonStream, opts.Structured = true, true
				opts.Prompt = fmt.Sprintf("Return an object with exactly two properties: nonce=%q and integer value=%d. No other properties.", nonce, value)
				expected = fmt.Sprintf(`{"nonce":%q,"value":%d}`, nonce, value)
			case "tools":
				opts.NonStream = true
				opts.Prompt = fmt.Sprintf("Call channel_probe with nonce=%q and value=%d. After the tool returns a receipt, reply with only the exact receipt string; do not call the tool again.", nonce, value)
				if i == 0 {
					opts.Tools = true
					expected = fmt.Sprintf(`{"nonce":%q,"value":%d}`, nonce, value)
				} else {
					opts.ToolCallID = tool.ID
					opts.ToolArguments = tool.Arguments
					opts.ToolResult = receipt
					expected = receipt
				}
			case "reasoning":
				q, err = newQuestion(i, false)
				if err != nil {
					return err
				}
				opts.Prompt, expected = q.Prompt, q.Expected
			case "speed":
				opts.Prompt = fmt.Sprintf("Verification %s. Write the integers 1 through 80 separated by spaces, without explanation.", uuid.NewString())
			case "cache":
				opts.Prefix, opts.CacheKey = prefix, "inspection-"+job.ID
				opts.Prompt = fmt.Sprintf("Ignore the reference entries above. Reply with only this round's verification code: %s-%d", nonce, i)
				expected = fmt.Sprintf("%s-%d", nonce, i)
			case "vision":
				opts.Prompt = "Count all red squares in the attached image. Reply with only the integer count and no explanation."
				opts.ImageDataURL, expected, err = inspectionImage()
				if err != nil {
					return err
				}
			case "context":
				expected = nonce
				kb := 8
				if o.Mode == "full" {
					kb = 32
				}
				opts.Prompt = "Read the whole reference and return the exact value for target_key, with no other text.\n" + cachePrefix(job.ID, kb/2) + "\ntarget_key=" + nonce + "\n" + cachePrefix(receipt, kb/2) + "\nWhat is target_key?"
			}
			q.Prompt, q.Expected = opts.Prompt, expected
			probe, used, probeErr := r.probe(ctx, current, t.Model, opts, true)
			if probe != nil {
				check.Requests++
				job.Result.Requests++
				job.Result.BudgetTokens += used
				job.Result.DurationMs += probe.DurationMs
				check.SampleIDs = append(check.SampleIDs, q.ID)
				job.Result.Items = append(job.Result.Items, Sample{CheckID: check.ID, QuestionID: q.ID, Family: check.ID, Prompt: q.Prompt, Expected: expected, Explanation: q.Explanation, ImageDataURL: opts.ImageDataURL, Probe: probe})
				if check.ID == "custom" {
					sample := &job.Result.Items[len(job.Result.Items)-1]
					sample.Name = o.Questions[i].Name
					sample.Match = o.Questions[i].Match
				}
			}
			if probeErr != nil || probe == nil {
				check.Verdict = "uncompleted"
				check.Reason = "transport_error"
				if probeErr != nil {
					check.Reason = probeErr.Error()
				}
				fatal = check.Reason
				job.Result.Reason = fatal
				break
			}
			if _, err = ensure(); err != nil {
				fatal = "configuration_changed"
				check.Verdict = "uncompleted"
				check.Reason = fatal
				job.Result.Reason = fatal
				break
			}
			sample := &job.Result.Items[len(job.Result.Items)-1]
			if !probe.Complete {
				check.Verdict, check.Reason = "uncompleted", probe.Reason
				if probe.Reason == "request_rejected" && (probe.HTTPStatus == 400 || probe.HTTPStatus == 404 || probe.HTTPStatus == 422) {
					check.Verdict = "unsupported"
				}
				break
			}
			pass := true
			switch check.ID {
			case "custom":
				sample.Correct, sample.GradeReason = gradeInspectionQuestion(o.Questions[i], probe.Text)
				if sample.Correct == nil {
					if check.Verdict == "passed" {
						check.Verdict, check.Reason = "unknown", "manual_review"
					}
				} else {
					pass = *sample.Correct
				}
			case "protocol_stream", "protocol_nonstream":
				pass = probe.WireValid && strings.TrimSpace(probe.Text) == expected
				if !probe.WireValid {
					check.Reason = "protocol_invalid"
				}
			case "structured":
				pass = inspectionObjectMatches(probe.Text, nonce, value)
			case "tools":
				if i == 0 {
					pass = len(probe.ToolCalls) == 1
					if pass {
						tool = probe.ToolCalls[0]
						pass = tool.Name == "channel_probe" && inspectionObjectMatches(tool.Arguments, nonce, value)
					}
				} else {
					pass = strings.TrimSpace(probe.Text) == receipt
				}
			case "reasoning", "vision":
				sample.Correct, sample.GradeReason = gradeAnswer(probe.Text, expected)
				if sample.Correct == nil {
					check.Verdict = "unknown"
					check.Reason = "answer_format"
				} else {
					pass = *sample.Correct
				}
			case "context", "cache":
				pass = strings.TrimSpace(probe.Text) == expected
			}
			if check.ID != "reasoning" && check.ID != "vision" && check.ID != "custom" {
				sample.Correct = &pass
			}
			if !pass {
				check.Verdict = "failed"
				if check.Reason == "" || check.Reason == "manual_review" {
					check.Reason = "unexpected_response"
				}
				if check.ID != "custom" {
					break
				}
			}
			if check.Verdict == "unknown" && check.ID != "custom" {
				break
			}
			if probe.FirstTokenMs != nil {
				latencies = append(latencies, *probe.FirstTokenMs)
			}
			if probe.UsageReported && probe.FirstTokenMs != nil && probe.DurationMs > *probe.FirstTokenMs {
				outputTokens += probe.OutputTokens
				generationMs += probe.DurationMs - *probe.FirstTokenMs
			}
			if check.ID == "cache" && i > 0 {
				input := probe.InputTokens
				if a.Platform == service.PlatformAnthropic {
					input += probe.CacheReadTokens + probe.CacheWriteTokens
				}
				if !probe.CacheReported || !probe.UsageReported || input <= 0 || probe.CacheReadTokens > input {
					cacheKnown = false
				} else {
					cacheInput += input
					cacheRead += probe.CacheReadTokens
				}
			}
		}
		if len(latencies) > 0 {
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			v := latencies[len(latencies)/2]
			check.MedianFirstTokenMs = &v
		}
		if generationMs > 0 {
			v := float64(outputTokens) * 1000 / float64(generationMs)
			check.TokensPerSecond = &v
		}
		if check.ID == "cache" && check.Verdict == "passed" {
			if !cacheKnown || cacheInput == 0 {
				check.Verdict = "unknown"
				check.Reason = "cache_unreported"
			} else {
				v := float64(cacheRead) * 100 / float64(cacheInput)
				check.CacheRatio = &v
				if cacheRead == 0 {
					check.Verdict = "not_observed"
				}
			}
		}
	}
	job.Verdict = "passed"
	for _, check := range job.Result.Checks {
		if check.Verdict == "failed" {
			job.Verdict = "suspected"
			break
		}
		if check.Verdict != "passed" {
			job.Verdict = "unknown"
		}
	}
	return nil
}

func inspectionIdentity(a *service.Account) string {
	raw, _ := json.Marshal(a.Credentials)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func inspectionObjectMatches(raw, nonce string, value int) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &object) != nil || len(object) != 2 {
		return false
	}
	var n string
	var v int
	return json.Unmarshal(object["nonce"], &n) == nil && json.Unmarshal(object["value"], &v) == nil && n == nonce && v == value
}

func inspectionProfileHash(a *service.Account, t Target, p Profile, bank []QuestionTemplate, o InspectionOptions) string {
	raw, _ := json.Marshal([]any{"inspection-v1", profileHash(a, t, p, bank), o})
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
