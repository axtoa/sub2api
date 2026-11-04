package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const auditVersion = "controlled-v1"

type AuditLane struct {
	AuditReference
	AccountID  int64  `json:"account_id"`
	Model      string `json:"model"`
	ConfigHash string `json:"config_hash"`
}
type AuditOptions struct {
	ReferenceIDs    []string    `json:"reference_ids,omitempty"`
	HistoricalRunID string      `json:"historical_run_id,omitempty"`
	Seed            string      `json:"seed"`
	References      []AuditLane `json:"references,omitempty"`
}

func (o AuditOptions) validate() error {
	if len(o.ReferenceIDs) > 3 || len(o.References) > 3 || len(o.Seed) > 64 {
		return invalid("每次最多选 3 个在线参照，种子最多 64 字符")
	}
	if o.Seed != "" && !externalToolID.MatchString(o.Seed) {
		return invalid("复现种子仅支持字母、数字、下划线和短横线")
	}
	seen := map[string]bool{}
	for _, id := range o.ReferenceIDs {
		if !externalToolID.MatchString(id) || seen[id] {
			return invalid("参照编号无效或重复")
		}
		seen[id] = true
	}
	if o.HistoricalRunID != "" {
		if _, err := uuid.Parse(o.HistoricalRunID); err != nil {
			return invalid("历史参照报告编号无效")
		}
	}
	return nil
}

type AuditSample struct {
	Lane     string                      `json:"lane"`
	CheckID  string                      `json:"check_id"`
	Pair     int                         `json:"pair"`
	Prompt   string                      `json:"prompt"`
	Expected string                      `json:"expected"`
	Correct  *bool                       `json:"correct"`
	Probe    *service.QualityProbeResult `json:"probe"`
	Reason   string                      `json:"reason,omitempty"`
}
type AuditComparison struct {
	Reference            string   `json:"reference"`
	Pair                 int      `json:"pair"`
	Status               string   `json:"status"`
	PrimaryInput         int64    `json:"primary_input"`
	ReferenceInput       int64    `json:"reference_input"`
	Difference           int64    `json:"difference"`
	ExtraPercent         *float64 `json:"extra_percent"`
	ReportedSharePercent *float64 `json:"reported_share_percent"`
}
type AuditResult struct {
	Version          string                 `json:"version"`
	Seed             string                 `json:"seed"`
	TolerancePercent int                    `json:"tolerance_percent"`
	Samples          []AuditSample          `json:"samples"`
	Comparisons      []AuditComparison      `json:"comparisons"`
	ManualReferences []ManualTokenReference `json:"manual_references,omitempty"`
	Fits             []AuditFit             `json:"fits,omitempty"`
}

type AuditFit struct {
	Reference   string  `json:"reference"`
	Samples     int     `json:"samples"`
	InputMin    int64   `json:"input_min"`
	InputMax    int64   `json:"input_max"`
	FixedExtra  float64 `json:"fixed_extra"`
	GrowthExtra float64 `json:"growth_extra"`
	RMSE        float64 `json:"rmse"`
}

func fitAudit(comparisons []AuditComparison) []AuditFit {
	groups := map[string][]AuditComparison{}
	for _, c := range comparisons {
		if c.Reference != "repeat" && (c.Status == "within_tolerance" || c.Status == "difference") {
			groups[c.Reference] = append(groups[c.Reference], c)
		}
	}
	out := []AuditFit{}
	for ref, items := range groups {
		lengths := map[int64]bool{}
		sx, sy := 0., 0.
		for _, c := range items {
			lengths[c.ReferenceInput] = true
			sx += float64(c.ReferenceInput)
			sy += float64(c.Difference)
		}
		if len(items) < 6 || len(lengths) < 3 {
			continue
		}
		mx, my := sx/float64(len(items)), sy/float64(len(items))
		xx, xy := 0., 0.
		for _, c := range items {
			x, y := float64(c.ReferenceInput)-mx, float64(c.Difference)-my
			xx += x * x
			xy += x * y
		}
		if xx == 0 {
			continue
		}
		slope := xy / xx
		intercept := my - slope*mx
		variance := 0.
		f := AuditFit{Reference: ref, Samples: len(items), InputMin: items[0].ReferenceInput, InputMax: items[0].ReferenceInput, FixedExtra: intercept, GrowthExtra: slope}
		for _, c := range items {
			res := float64(c.Difference) - intercept - slope*float64(c.ReferenceInput)
			variance += res * res
			f.InputMin = min(f.InputMin, c.ReferenceInput)
			f.InputMax = max(f.InputMax, c.ReferenceInput)
		}
		f.RMSE = math.Sqrt(variance / float64(len(items)))
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Reference < out[j].Reference })
	return out
}

func (r *Runner) prepareAudit(ctx context.Context, o *InspectionOptions, t Target) error {
	if o.Audit == nil {
		return nil
	}
	cfg, err := r.Store.Settings(ctx)
	if err != nil {
		return err
	}
	settings, err := r.Store.Assurance(ctx)
	if err != nil {
		return err
	}
	input := o.Audit
	input.References = nil // only the server creates route snapshots
	if input.Seed == "" {
		input.Seed = uuid.NewString()
	}
	if input.HistoricalRunID != "" {
		old, err := scanRun(r.Store.DB.QueryRowContext(ctx, "SELECT "+runColumns+" FROM quality_monitor_runs WHERE id=$1", input.HistoricalRunID))
		if err != nil {
			return err
		}
		if old.Status != "finished" || old.Result == nil || old.Result.Audit == nil || old.Result.Audit.Version != auditVersion {
			return invalid("历史参照需要已完成的同版本审计报告")
		}
		oldTarget, err := r.Store.Target(ctx, old.TargetID)
		if err != nil {
			return err
		}
		if oldTarget.Model != t.Model || oldTarget.Platform != t.Platform {
			return invalid("历史参照必须使用相同平台与请求模型")
		}
		input.Seed = old.Result.Audit.Seed
	}
	seenTargets := map[string]bool{t.ID: true}
	for _, id := range input.ReferenceIDs {
		found := false
		for _, ref := range settings.References {
			if ref.ID != id {
				continue
			}
			found = true
			target, err := r.Store.Target(ctx, ref.TargetID)
			if err != nil {
				return err
			}
			if seenTargets[target.ID] || target.AccountID == t.AccountID || target.Platform != t.Platform || target.Model != t.Model {
				return invalid("参照需选择同平台、同请求模型的其他账号，不可重复")
			}
			seenTargets[target.ID] = true
			account, err := r.Accounts.GetByID(ctx, target.AccountID)
			if err != nil {
				return err
			}
			if !service.QualityProbeSupported(account, target.Model) {
				return invalid("参照渠道当前不支持检验")
			}
			input.References = append(input.References, AuditLane{AuditReference: ref, AccountID: target.AccountID, Model: target.Model, ConfigHash: profileHash(account, target, t.Effective(cfg), cfg.QuestionBank)})
		}
		if !found {
			return invalid("参照配置已变化，请重新载入")
		}
	}
	return nil
}

// Repetition measures observed variability, never a cheap-model mixing rate.
func auditTask(seed, check string, index int) (string, string) {
	group := index / 3
	if check == "usage_audit" {
		group = index / 2
	}
	hash := sha256.Sum256([]byte(auditVersion + seed + check + fmt.Sprint(group)))
	nonce := hex.EncodeToString(hash[:8])
	if check == "usage_audit" {
		size := []int{128, 2048, 8192}[index/2]
		filler := strings.Repeat("alpha beta gamma delta\n", size/23+2)[:size]
		return "Reference text (do not repeat):\n" + filler + "\nReply with exactly this code and no other text: " + nonce, nonce
	}
	var seedBytes [16]byte
	copy(seedBytes[:], hash[:16])
	question := questionFromSeed(index/3, false, seedBytes)
	return question.Prompt, question.Expected
}

func auditInput(p *service.QualityProbeResult) int64 {
	n := p.InputTokens
	if p.Protocol == "anthropic" {
		n += p.CacheReadTokens + p.CacheWriteTokens
	}
	return n
}
func compareAudit(a, b AuditSample, tolerance int) AuditComparison {
	c := AuditComparison{Reference: b.Lane, Pair: a.Pair, Status: "incomparable"}
	p, q := a.Probe, b.Probe
	if a.Reason != "" || b.Reason != "" || p == nil || q == nil || !p.Complete || !q.Complete {
		return c
	}
	if p.RequestHash == "" || p.RequestHash != q.RequestHash || p.Protocol != q.Protocol || !p.ControlledInput || !q.ControlledInput {
		return c
	}
	if !p.UsageReported || !q.UsageReported || len(p.UsageAnomalies) > 0 || len(q.UsageAnomalies) > 0 {
		c.Status = "usage_missing"
		return c
	}
	c.PrimaryInput, c.ReferenceInput = auditInput(p), auditInput(q)
	c.Difference = c.PrimaryInput - c.ReferenceInput
	if c.ReferenceInput <= 0 {
		return c
	}
	extra := float64(c.Difference) * 100 / float64(c.ReferenceInput)
	c.ExtraPercent = &extra
	if c.PrimaryInput > 0 {
		share := float64(c.Difference) * 100 / float64(c.PrimaryInput)
		c.ReportedSharePercent = &share
	}
	c.Status = "within_tolerance"
	if math.Abs(float64(c.Difference)) > 32 && math.Abs(extra) > float64(tolerance) {
		c.Status = "difference"
	}
	return c
}

func (r *Runner) executeAudit(ctx context.Context, job *Run, t Target, cfg Settings, p Profile, check *InspectionCheck, ensure func() (*service.Account, error)) error {
	options := job.Inspection.Audit
	if job.Result.Audit == nil {
		settings, err := r.Store.Assurance(ctx)
		if err != nil {
			return err
		}
		job.Result.Audit = &AuditResult{Version: auditVersion, Seed: options.Seed, TolerancePercent: settings.Policy.UsageTolerancePercent, Samples: []AuditSample{}, Comparisons: []AuditComparison{}}
		job.Result.Audit.ManualReferences = settings.ManualReferences
	}
	out := job.Result.Audit
	historical := []AuditSample{}
	if options.HistoricalRunID != "" {
		old, err := scanRun(r.Store.DB.QueryRowContext(ctx, "SELECT "+runColumns+" FROM quality_monitor_runs WHERE id=$1", options.HistoricalRunID))
		if err == nil && old.Result != nil && old.Result.Audit != nil {
			historical = old.Result.Audit.Samples
		}
	}
	identities := map[int64]string{}
	check.Verdict = "unknown"
	check.Reason = "limited_evidence"
	for i := 0; i < inspectionCount(check.ID, job.Inspection.Mode); i++ {
		prompt, expected := auditTask(options.Seed, check.ID, i)
		var primary AuditSample
		for lane := -1; lane < len(options.References); lane++ {
			current, err := ensure()
			if err != nil {
				job.Result.Reason = "configuration_changed"
				check.Reason = job.Result.Reason
				check.Verdict = "uncompleted"
				return nil
			}
			model, id := t.Model, "primary"
			var ref *AuditLane
			if lane >= 0 {
				ref = &options.References[lane]
				id, model = ref.ID, ref.Model
				target, e := r.Store.Target(ctx, ref.TargetID)
				if e != nil {
					err = e
				} else {
					current, err = r.Accounts.GetByID(ctx, ref.AccountID)
				}
				if err == nil && (current == nil || !service.QualityProbeSupported(current, model) || profileHash(current, target, p, cfg.QuestionBank) != ref.ConfigHash) {
					err = errors.New("configuration_changed")
				}
				if err == nil {
					identity := inspectionIdentity(current)
					if identities[current.ID] != "" && identities[current.ID] != identity {
						err = errors.New("configuration_changed")
					} else {
						identities[current.ID] = identity
					}
				}
			}
			sample := AuditSample{Lane: id, CheckID: check.ID, Pair: i, Prompt: prompt, Expected: expected}
			if err != nil {
				sample.Reason = "reference_changed"
				out.Samples = append(out.Samples, sample)
				if check.ID == "usage_audit" {
					out.Comparisons = append(out.Comparisons, compareAudit(primary, sample, out.TolerancePercent))
				}
				continue
			}
			if err = r.Store.Renew(ctx, *job, p, job.ProfileHash); err != nil {
				return err
			}
			opts := service.QualityProbeOptions{Audit: true, Inspection: true, Prompt: prompt, MaxOutputTokens: p.MaxOutputTokens, ReasoningEffort: p.ReasoningEffort}
			probe, used, probeErr := r.probe(ctx, current, model, opts, true)
			sample.Probe = probe
			if probe != nil {
				job.Result.Requests++
				check.Requests++
				job.Result.BudgetTokens += used
				job.Result.DurationMs += probe.DurationMs
			}
			if probeErr != nil {
				sample.Reason = "request_unavailable"
			}
			if _, e := ensure(); e != nil {
				job.Result.Reason = "configuration_changed"
				sample.Reason = "configuration_changed"
			}
			if ref != nil {
				updated, e := r.Accounts.GetByID(ctx, ref.AccountID)
				target, te := r.Store.Target(ctx, ref.TargetID)
				if e != nil || te != nil || updated == nil || inspectionIdentity(updated) != identities[ref.AccountID] || profileHash(updated, target, p, cfg.QuestionBank) != ref.ConfigHash {
					sample.Reason = "reference_changed"
					sample.Probe = nil
				}
			}
			if sample.Reason == "" && probe != nil && probe.Complete {
				pass := strings.TrimSpace(probe.Text) == expected
				sample.Correct = &pass
			}
			if lane < 0 {
				primary = sample
				if probe != nil {
					qid := uuid.NewString()
					check.SampleIDs = append(check.SampleIDs, qid)
					job.Result.Items = append(job.Result.Items, Sample{CheckID: check.ID, QuestionID: qid, Family: check.ID, Prompt: prompt, Expected: expected, Correct: sample.Correct, Probe: probe})
				}
			} else if check.ID == "usage_audit" {
				out.Comparisons = append(out.Comparisons, compareAudit(primary, sample, out.TolerancePercent))
			}
			out.Samples = append(out.Samples, sample)
			if job.Result.Reason == "configuration_changed" {
				check.Verdict = "uncompleted"
				check.Reason = job.Result.Reason
				return nil
			}
		}
		if check.ID == "usage_audit" && i%2 == 1 {
			for _, previous := range out.Samples {
				if previous.Lane == "primary" && previous.CheckID == check.ID && previous.Pair == i-1 {
					previous.Lane = "repeat"
					out.Comparisons = append(out.Comparisons, compareAudit(primary, previous, 0))
					break
				}
			}
		}
		if check.ID == "usage_audit" && primary.Probe != nil {
			for _, ref := range out.ManualReferences {
				if ref.RequestHash != primary.Probe.RequestHash || ref.Protocol != primary.Probe.Protocol || ref.Model != t.Model || time.Since(ref.CapturedAt) > 30*24*time.Hour {
					continue
				}
				manual := AuditSample{Lane: "manual:" + ref.ID, Pair: i, Probe: &service.QualityProbeResult{RequestHash: ref.RequestHash, Protocol: ref.Protocol, ControlledInput: true, Complete: true, UsageReported: true, InputTokens: ref.InputTokens}}
				out.Comparisons = append(out.Comparisons, compareAudit(primary, manual, out.TolerancePercent))
			}
		}
		if check.ID == "usage_audit" {
			for _, old := range historical {
				if old.Lane == "primary" && old.CheckID == check.ID && old.Pair == i {
					old.Lane = "history:" + options.HistoricalRunID
					out.Samples = append(out.Samples, old)
					out.Comparisons = append(out.Comparisons, compareAudit(primary, old, out.TolerancePercent))
					break
				}
			}
		}
	}
	if check.ID == "stability_audit" {
		graded, correct := 0, 0
		for _, s := range out.Samples {
			if s.CheckID == check.ID && s.Lane == "primary" && s.Correct != nil {
				graded++
				if *s.Correct {
					correct++
				}
			}
		}
		if graded == 9 {
			check.Verdict = "passed"
			check.Reason = "observed_consistency"
			if correct != graded {
				check.Verdict = "failed"
				check.Reason = "capability_variation"
			}
		}
	} else {
		out.Fits = fitAudit(out.Comparisons)
		valid, diff := 0, 0
		for _, c := range out.Comparisons {
			if c.Reference == "repeat" {
				continue
			}
			if c.Status == "difference" {
				diff++
			}
			if c.Status == "difference" || c.Status == "within_tolerance" {
				valid++
			}
		}
		if valid > 0 {
			check.Reason = "within_tolerance"
			check.Verdict = "passed"
			if diff > 0 {
				check.Reason = "usage_difference"
				check.Verdict = "unknown"
			}
		}
	}
	return nil
}
