//go:build integration

package quality

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type auditTestAccounts map[int64]*service.Account

func (a auditTestAccounts) GetByID(_ context.Context, id int64) (*service.Account, error) {
	return a[id], nil
}

type auditTestProber struct {
	calls map[int64]int
	after func(int64)
}

func (p *auditTestProber) RunQualityProbe(_ context.Context, id int64, _ string, o service.QualityProbeOptions) (*service.QualityProbeResult, error) {
	p.calls[id]++
	if p.after != nil {
		p.after(id)
	}
	n := int64(len(o.Prompt) / 4)
	if id == 1 {
		n += 100
	}
	expected := ""
	for _, check := range []string{"usage_audit", "stability_audit"} {
		for i := 0; i < inspectionCount(check, "quick"); i++ {
			prompt, answer := auditTask("stable", check, i)
			if prompt == o.Prompt {
				expected = answer
			}
		}
	}
	return &service.QualityProbeResult{Text: expected, Complete: true, HTTPStatus: 200, UsageReported: true, ControlledInput: true, RequestHash: fmt.Sprintf("%x", sha256.Sum256([]byte(o.Prompt))), Protocol: "chat", InputTokens: n, OutputTokens: 5}, nil
}

func TestQualityAssurancePersistenceAndScoringHistory(t *testing.T) {
	db := qualityTestDB(t)
	store := &Store{DB: db}
	ctx := context.Background()
	settings, err := store.Assurance(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, settings.Version)
	saved, err := store.SaveAssurance(ctx, settings)
	require.NoError(t, err)
	require.Equal(t, 2, saved.Version)
	_, err = store.SaveAssurance(ctx, settings)
	require.Error(t, err, "stale editors cannot overwrite settings")
	cfg, err := store.Settings(ctx)
	require.NoError(t, err)
	target, err := store.SaveTarget(ctx, Target{AccountID: 1, Model: "text-model", Enabled: true}, cfg)
	require.NoError(t, err)
	id := uuid.NewString()
	result := Result{Reason: "", Requests: 1, Items: []Sample{{Prompt: "private synthetic prompt", Expected: "secret answer", Correct: boolPtr(true), Probe: &service.QualityProbeResult{Complete: true, Text: "private response", HTTPStatus: 200}}}}
	raw, _ := json.Marshal(result)
	_, err = db.Exec("INSERT INTO quality_monitor_runs(id,target_id,kind,status,manual,result,created_at,finished_at) VALUES($1,$2,'quality','finished',TRUE,$3,NOW()-INTERVAL '40 days',NOW()-INTERVAL '40 days')", id, target.ID, string(raw))
	require.NoError(t, err)
	evidence := AssuranceEvidence{ID: uuid.NewString(), TargetID: target.ID, RunID: id, Kind: "privacy", Status: "confirmed", Note: "manual evidence", CreatedAt: time.Now()}
	detail, _ := json.Marshal(evidence)
	_, err = db.Exec("INSERT INTO quality_assurance_evidence(id,run_id,kind,status,detail) VALUES($1,$2,'privacy','confirmed',$3)", evidence.ID, id, string(detail))
	require.NoError(t, err)
	require.NoError(t, store.Cleanup(ctx))
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM quality_monitor_runs WHERE id=$1", id).Scan(&count))
	require.Equal(t, 1, count, "cleanup retains attested evidence")
	history, err := store.scoreRuns(ctx, time.Now().Add(-100*24*time.Hour))
	require.NoError(t, err)
	require.Len(t, history[target.ID], 1)
	require.Empty(t, history[target.ID][0].Result.Items[0].Prompt)
	require.Empty(t, history[target.ID][0].Result.Items[0].Probe.Text)
	require.True(t, *history[target.ID][0].Result.Items[0].Correct)
	corrections, err := store.AssuranceEvidence(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, corrections, 1)
}

func boolPtr(v bool) *bool { return &v }

func TestQualityAuditRunnerOptionalAndChangedReferences(t *testing.T) {
	db := qualityTestDB(t)
	store := &Store{DB: db}
	ctx := context.Background()
	_, err := db.Exec("INSERT INTO accounts(id,name,platform,type) VALUES(2,'reference','openai','apikey'),(3,'another','openai','apikey')")
	require.NoError(t, err)
	cfg, err := store.Settings(ctx)
	require.NoError(t, err)
	accounts := auditTestAccounts{}
	targets := []Target{}
	for _, id := range []int64{1, 2, 3} {
		accounts[id] = &service.Account{ID: id, Platform: "openai", Type: "apikey", Concurrency: 1, Credentials: map[string]any{"api_key": fmt.Sprint(id), "base_url": "https://example.com"}}
		target, e := store.SaveTarget(ctx, Target{AccountID: id, Model: "text-model", Enabled: true}, cfg)
		require.NoError(t, e)
		target,e=store.Target(ctx,target.ID);require.NoError(t,e)
 targets = append(targets, target)
	}
	prober := &auditTestProber{calls: map[int64]int{}}
	runner := &Runner{Store: store, Accounts: accounts, Prober: prober, Slots: &testSlots{acquired: true}, Location: time.UTC}
	settings, err := store.Assurance(ctx)
	require.NoError(t, err)
	settings.References = []AuditReference{{ID: "trusted", TargetID: targets[1].ID, Name: "trusted relay", Source: "trusted"}, {ID: "other", TargetID: targets[2].ID, Name: "another relay", Source: "other"}}
	_, err = store.SaveAssurance(ctx, settings)
	require.NoError(t, err)
	options := InspectionOptions{Mode: "quick", Checks: []string{"usage_audit", "stability_audit"}, Audit: &AuditOptions{Seed: "stable"}}
	require.NoError(t, runner.prepareAudit(ctx, &options, targets[0]))
	job, err := store.QueueInspection(ctx, targets[0].ID, options)
	require.NoError(t, err)
	claimed, err := store.Claim(ctx)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	require.NoError(t, runner.execute(ctx, *claimed, cfg))
	done, err := scanRun(db.QueryRow("SELECT "+runColumns+" FROM quality_monitor_runs WHERE id=$1", job.ID))
	require.NoError(t, err)
	require.Equal(t, 15, done.Result.Requests)
	require.Len(t, done.Result.Audit.Comparisons, 3)
	require.Equal(t, "unknown", done.Result.Checks[0].Verdict, "repeatability is not external proof")
	require.Equal(t, "passed", done.Result.Checks[1].Verdict)
	require.Equal(t, 0, prober.calls[2])

	options.Audit.ReferenceIDs = []string{"trusted", "other"}
	require.NoError(t, runner.prepareAudit(ctx, &options, targets[0]))
	prober.after = func(id int64) {
		if id == 2 {
			accounts[2].Credentials["api_key"] = "rotated"
		}
	}
	job, err = store.QueueInspection(ctx, targets[0].ID, options)
	require.NoError(t, err)
	claimed, err = store.Claim(ctx)
	require.NoError(t, err)
	require.NoError(t, runner.execute(ctx, *claimed, cfg))
	done, err = scanRun(db.QueryRow("SELECT "+runColumns+" FROM quality_monitor_runs WHERE id=$1", job.ID))
	require.NoError(t, err)
	require.Equal(t, 1, prober.calls[2], "changed reference is never silently used again")
	require.Equal(t, 15, prober.calls[3], "another reference continues")
	require.Equal(t, 30, prober.calls[1])
	for _, c := range done.Result.Audit.Comparisons {
		if c.Reference == "trusted" {
			require.Equal(t, "incomparable", c.Status)
		}
	}
	require.NotContains(t, fmt.Sprint(done), "rotated")
	require.True(t, strings.Contains(done.Result.Audit.Version, "controlled"))
}
