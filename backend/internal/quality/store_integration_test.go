//go:build integration

package quality

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func qualityTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("QUALITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("QUALITY_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("quality_test_%d", time.Now().UnixNano())
	_, err = db.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, dropErr)
		require.NoError(t, db.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	pool, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	pool.SetMaxOpenConns(10)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	for _, name := range []string{"001_init.sql", "033_ops_monitoring_vnext.sql", "077_add_usage_log_requested_model.sql", "079_ops_error_logs_add_endpoint_fields.sql", "244_quality_monitor.sql", "244_quality_monitor.sql", "245_quality_monitor_accounts.sql", "245_quality_monitor_accounts.sql"} {
		migration, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		// pg_trgm is already installed in public in the preview database; only
		// qualify its operator class, leaving all test tables in our own schema.
		_, err = pool.Exec(strings.ReplaceAll(string(migration), "gin_trgm_ops", "public.gin_trgm_ops"))
		require.NoError(t, err, name)
	}
	// This column was added by the original ent/schema parity migration.
	_, err = pool.Exec(`ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS first_token_ms BIGINT;
 ALTER TABLE accounts ADD COLUMN IF NOT EXISTS parent_account_id BIGINT;
 INSERT INTO accounts(id,name,platform,type) VALUES(1,'test account','openai','apikey');
 INSERT INTO users(id,email,password_hash) VALUES(1,'test@example.com','unused');
 INSERT INTO api_keys(id,user_id,key,name) VALUES(1,1,'unused','test');`)
	require.NoError(t, err)
	return pool
}

func TestQualityPostgresBudgetsQueueAndSchedules(t *testing.T) {
	db := qualityTestDB(t)
	s := &Store{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cfg, err := s.Settings(ctx)
	require.NoError(t, err)
	cfg.DailyRequests = 5
	cfg.DailyTokens = 10000
	cfg, err = s.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	target, err := s.SaveTarget(ctx, Target{AccountID: 1, Model: "test-model", Enabled: true}, cfg)
	require.NoError(t, err)
	_, err = s.SaveTarget(ctx, Target{AccountID: 1, Model: "test-model", Enabled: true}, cfg)
	require.True(t, isConflict(err))
	var reserved atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, e := s.Reserve(ctx, "2026-10-23", 2000)
			require.NoError(t, e)
			if ok {
				reserved.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 5, reserved.Load())
	budget, err := s.Budget(ctx, cfg, time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC), time.UTC)
	require.NoError(t, err)
	require.True(t, budget.Limited)
	require.EqualValues(t, 10000, budget.Tokens)
	require.NoError(t, s.Settle(ctx, "2026-10-23", 2000, 50, true))
	require.NoError(t, s.Settle(ctx, "2026-10-23", 2000, 0, false))
	cfg.DailyRequests = 20
	cfg, err = s.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	budget, err = s.Budget(ctx, cfg, time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC), time.UTC)
	require.NoError(t, err)
	require.Equal(t, 5, budget.Requests)
	require.EqualValues(t, 8050, budget.Tokens)
	require.Equal(t, 4, budget.UnconfirmedRequests)
	old := cfg
	cfg.QualityIntervalMinutes = 180
	cfg, err = s.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	_, err = s.SaveSettings(ctx, old)
	require.True(t, isConflict(err))
	var queued atomic.Int64
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Queue(ctx, target.ID, "quality", false)
			if e == nil {
				queued.Add(1)
			} else {
				require.True(t, isConflict(e))
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, queued.Load())
	var claimed atomic.Int64
	var job *Run
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.Claim(ctx)
			require.NoError(t, e)
			if r != nil {
				claimed.Add(1)
				mu.Lock()
				job = r
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, claimed.Load())
	require.NoError(t, s.Renew(ctx, *job, cfg.Profile, "hash"))
	job.Verdict = "passed"
	job.Result = &Result{Items: []Sample{}}
	target, err = s.Target(ctx, target.ID)
	require.NoError(t, err)
	target.Enabled = false
	target, err = s.SaveTarget(ctx, target, cfg)
	require.NoError(t, err)
	target, err = s.Target(ctx, target.ID)
	require.NoError(t, err)
	editedNext := target.NextQualityAt
	require.NoError(t, s.Finish(ctx, *job, time.Now().Add(999*time.Hour), 1, target.Version-1, cfg.Version))
	after, err := s.Target(ctx, target.ID)
	require.NoError(t, err)
	require.WithinDuration(t, editedNext, after.NextQualityAt, time.Millisecond)
	require.False(t, after.Enabled)
	history, err := s.History(ctx, target.ID, 60)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, "passed", history[0].Verdict)
	_, err = s.Queue(ctx, target.ID, "availability", false)
	require.NoError(t, err)
	expired, err := s.Claim(ctx)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE quality_monitor_runs SET lease_until=NOW()-INTERVAL '1 minute' WHERE id=$1`, expired.ID)
	require.NoError(t, err)
	nothing, err := s.Claim(ctx)
	require.NoError(t, err)
	require.Nil(t, nothing)
	history, err = s.History(ctx, target.ID, 60)
	require.NoError(t, err)
	require.Equal(t, "interrupted", history[0].Result.Reason)
	after, err = s.Target(ctx, target.ID)
	require.NoError(t, err)
	require.True(t, after.NextAvailabilityAt.After(time.Now().Add(59*time.Minute)), "expired jobs must not be rescheduled immediately")
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,requested_model,input_tokens,cache_read_tokens,first_token_ms) VALUES(1,1,1,'upstream','test-model',100,50,125);
 INSERT INTO ops_error_logs(account_id,model,requested_model,error_phase,error_type,status_code,upstream_status_code) VALUES(1,'upstream','test-model','upstream','test',502,502);`)
	require.NoError(t, err)
	metrics, err := s.Metrics(ctx, target, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.EqualValues(t, 1, metrics.Successes)
	require.EqualValues(t, 1, metrics.Errors)
	require.EqualValues(t, 50, metrics.BilledCacheTokens)
	targets, err := s.Targets(ctx)
	require.NoError(t, err)
	require.NoError(t, s.Enrich(ctx, targets, time.Now().Add(-time.Hour)))
	require.EqualValues(t, metrics, targets[0].Metrics)
	require.NotNil(t, targets[0].LatestQuality)
	require.NoError(t, s.Cleanup(ctx))
}

func TestQualityPostgresCleanupKeepsDailyProbeDenominator(t *testing.T) {
	db := qualityTestDB(t)
	s := &Store{DB: db}
	ctx := context.Background()
	cfg, err := s.Settings(ctx)
	require.NoError(t, err)
	target, err := s.SaveTarget(ctx, Target{AccountID: 1, Model: "text-model", Enabled: true}, cfg)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO quality_monitor_runs(id,target_id,kind,status,manual,verdict,result,created_at,finished_at)
 SELECT ('00000000-0000-4000-8000-'||LPAD(n::text,12,'0'))::uuid,$1,'availability','finished',TRUE,
 CASE WHEN n IN (200,211) THEN 'unavailable' ELSE 'passed' END,'{"reason":"","requests":1}',
 CASE WHEN n<=200 THEN NOW()-n*INTERVAL '1 minute' ELSE NOW()-INTERVAL '25 hours' END,
 CASE WHEN n<=200 THEN NOW()-n*INTERVAL '1 minute' WHEN n=211 THEN NOW() ELSE NOW()-INTERVAL '25 hours' END
 FROM generate_series(1,211) n`, target.ID)
	require.NoError(t, err)
	require.NoError(t, s.Cleanup(ctx))
	var retained int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_monitor_runs`).Scan(&retained))
	require.Equal(t, 201, retained, "all probes completed in the statistics window survive cleanup, including delayed jobs")
	targets, err := s.Targets(ctx)
	require.NoError(t, err)
	require.NoError(t, s.Enrich(ctx, targets, time.Now().Add(-24*time.Hour)))
	require.Equal(t, 199, targets[0].ProbeSuccesses)
	require.Equal(t, 2, targets[0].ProbeFailures)
	require.Len(t, targets[0].Recent, 180, "chart payload stays bounded")
	require.Equal(t, "unavailable", targets[0].Recent[0].Verdict, "the chart orders by completion, not enqueue time")
}

type testAccounts struct{ account *service.Account }

func (a *testAccounts) GetByID(context.Context, int64) (*service.Account, error) {
	return a.account, nil
}

type testSlots struct {
	acquired bool
	releases int
}

func (s *testSlots) AcquireAccountSlot(context.Context, int64, int) (*service.AcquireResult, error) {
	return &service.AcquireResult{Acquired: s.acquired, ReleaseFunc: func() { s.releases++ }}, nil
}

type testProber struct {
	calls int
	mode  string
	after func()
}

func (p *testProber) RunQualityProbe(_ context.Context, _ int64, _ string, opts service.QualityProbeOptions) (*service.QualityProbeResult, error) {
	p.calls++
	// Return a valid but intentionally incorrect JSON for capability anomaly tests.
	nonce := strings.Split(strings.Split(opts.Prompt, `"check":"`)[1], `"`)[0]
	result := &service.QualityProbeResult{Text: fmt.Sprintf(`{"answer":-99999,"check":%q}`, nonce), Complete: true, UsageReported: true, InputTokens: 20, OutputTokens: 10, HTTPStatus: 200}
	if strings.HasPrefix(p.mode, "cache_") || p.mode == "availability" {
		var a, b int
		_, err := fmt.Sscanf(opts.Prompt, "Compute %d + %d.", &a, &b)
		if err != nil {
			return nil, err
		}
		result.Text = fmt.Sprintf(`{"answer":%d,"check":%q}`, a+b, nonce)
		result.CacheReported = p.mode != "cache_unreported"
		if strings.HasPrefix(p.mode, "cache_") {
			result.InputTokens = 5000
		}
		if p.mode == "cache_hit" && p.calls > 1 {
			result.CacheReadTokens = 1024
		}
		if p.mode == "cache_invalid" {
			result.InputTokens = 0
		}
		if p.mode == "cache_invalid_read" {
			result.CacheReadTokens = result.InputTokens + 1
		}
	}
	if p.mode == "incomplete" {
		result.Complete = false
		result.Reason = "incomplete"
	}
	if p.after != nil {
		p.after()
	}
	return result, nil
}

func TestQualityPostgresCacheBudgetAndTraffic(t *testing.T) {
	db := qualityTestDB(t)
	ctx := context.Background()
	r := NewRunner(db, nil, nil, nil, "Asia/Shanghai")
	account := &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: "active", Schedulable: true, Concurrency: 1}
	r.Accounts = &testAccounts{account}
	r.Slots = &testSlots{acquired: true}
	cfg, err := r.Store.Settings(ctx)
	require.NoError(t, err)
	target, err := r.Store.SaveTarget(ctx, Target{AccountID: 1, Model: "text-model", Enabled: true}, cfg)
	require.NoError(t, err)
	execute := func(kind string, manual bool) Run {
		_, e := r.Store.Queue(ctx, target.ID, kind, manual)
		require.NoError(t, e)
		job, e := r.Store.Claim(ctx)
		require.NoError(t, e)
		require.NoError(t, r.execute(ctx, *job, cfg))
		rows, e := r.Store.History(ctx, target.ID, 1)
		require.NoError(t, e)
		return rows[0]
	}
	for _, tc := range []struct {
		mode, verdict, reason string
		calls                 int
	}{
		{"cache_hit", "cache_hit", "", 2}, {"cache_miss", "not_observed", "", 3}, {"cache_unreported", "unknown", "cache_unreported", 2},
		{"cache_invalid", "unknown", "cache_usage_invalid", 2}, {"cache_invalid_read", "unknown", "cache_usage_invalid", 2},
	} {
		probe := &testProber{mode: tc.mode}
		r.Prober = probe
		result := execute("cache", true)
		require.Equal(t, tc.verdict, result.Verdict)
		require.Equal(t, tc.reason, result.Result.Reason)
		require.Equal(t, tc.calls, probe.calls)
		if tc.verdict == "cache_hit" {
			require.EqualValues(t, 5000, result.Result.CacheInputTokens, "warmup tokens do not count toward the cache ratio")
			require.EqualValues(t, 1024, result.Result.CacheReadTokens)
		}
		if tc.reason == "cache_usage_invalid" {
			require.False(t, result.Result.CacheReported)
		}
	}
	probe := &testProber{mode: "availability"}
	r.Prober = probe
	available := execute("availability", true)
	require.Equal(t, "passed", available.Verdict)
	require.Equal(t, 1, probe.calls)
	_, err = db.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,requested_model,first_token_ms) VALUES(1,1,1,'upstream','text-model',125)`)
	require.NoError(t, err)
	traffic := execute("availability", false)
	require.Equal(t, "real_traffic", traffic.Result.Reason)
	require.Equal(t, 1, probe.calls, "real traffic avoids an extra upstream call")
	budget, err := r.Store.Budget(ctx, cfg, time.Now(), r.Location)
	require.NoError(t, err)
	require.Zero(t, budget.Requests, "manual checks do not consume the automatic budget")
	require.Equal(t, 12, budget.ManualRequests)
	cfg.DailyRequests = 1
	cfg, err = r.Store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	ok, err := r.Store.Reserve(ctx, budget.Day, 1000)
	require.NoError(t, err)
	require.True(t, ok)
	limited := execute("availability", true)
	require.Equal(t, "passed", limited.Verdict, "manual checks bypass the exhausted automatic budget")
	require.Equal(t, 1, limited.Result.Requests)
	require.Equal(t, 2, probe.calls)
	// Turning automatic checks off does not disable explicit manual checks.
	cfg.AutoEnabled = false
	cfg.DailyRequests = 100
	cfg, err = r.Store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE quality_monitor_targets SET next_quality_at=NOW()-INTERVAL '1 hour'`)
	require.NoError(t, err)
	require.NoError(t, r.cycle(ctx))
	var pending int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_monitor_runs WHERE status IN ('queued','running')`).Scan(&pending))
	require.Zero(t, pending)
	manual := execute("availability", true)
	require.Equal(t, "passed", manual.Verdict)
	require.Equal(t, 3, probe.calls)
}

func TestQualityPostgresRunnerObservationAndGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := qualityTestDB(t)
	r := NewRunner(db, nil, nil, nil, "Asia/Shanghai")
	ctx := context.Background()
	cfg, err := r.Store.Settings(ctx)
	require.NoError(t, err)
	account := &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: "active", Schedulable: true, Concurrency: 1, Credentials: map[string]any{"base_url": "https://example.com"}}
	r.Accounts = &testAccounts{account}
	slots := &testSlots{acquired: true}
	r.Slots = slots
	prober := &testProber{}
	r.Prober = prober
	target, err := r.Store.SaveTarget(ctx, Target{AccountID: 1, Model: "model", Enabled: true}, cfg)
	require.NoError(t, err)
	execute := func() *Run {
		_, e := r.Store.Queue(ctx, target.ID, "quality", false)
		require.NoError(t, e)
		job, e := r.Store.Claim(ctx)
		require.NoError(t, e)
		require.NoError(t, r.execute(ctx, *job, cfg))
		items, e := r.Store.History(ctx, target.ID, 1)
		require.NoError(t, e)
		return &items[0]
	}
	first := execute()
	require.Equal(t, "suspected", first.Verdict)
	require.Equal(t, 3, prober.calls)
	require.Equal(t, 3, slots.releases)
	current, err := r.Store.Target(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, 1, current.RetryAttempt)
	second := execute()
	require.Equal(t, "degraded", second.Verdict)
	require.Equal(t, 6, prober.calls)
	current, err = r.Store.Target(ctx, target.ID)
	require.NoError(t, err)
	require.Zero(t, current.RetryAttempt)
	prober.mode = "incomplete"
	third := execute()
	require.Equal(t, "unknown", third.Verdict)
	require.Equal(t, 0, third.Result.Graded)
	prober.mode = ""
	before := prober.calls
	account.Schedulable = false
	paused := execute()
	require.Equal(t, "account_paused", paused.Result.Reason)
	require.Equal(t, before, prober.calls)
	account.Schedulable = true
	slots.acquired = false
	capacity := execute()
	require.Equal(t, "capacity_unavailable", capacity.Result.Reason)
	require.Equal(t, before, prober.calls)
	slots.acquired = true
	prober.after = func() { account.Credentials["base_url"] = "https://changed.example.com" }
	changed := execute()
	require.Equal(t, "configuration_changed", changed.Result.Reason)
	require.Equal(t, before+1, prober.calls)
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM accounts WHERE id=1`).Scan(&status))
	require.Equal(t, "active", status)
	budget, err := r.Store.Budget(ctx, cfg, time.Now(), r.Location)
	require.NoError(t, err)
	require.Equal(t, 8, budget.Requests)
	require.Greater(t, budget.UnconfirmedRequests, 0)
	finalRoundCalls := 0
	prober.after = func() {
		finalRoundCalls++
		if finalRoundCalls == 3 {
			account.Credentials["base_url"] = "https://edited-during-final-request.example.com"
		}
	}
	lastChanged := execute()
	require.Equal(t, 3, finalRoundCalls)
	require.Equal(t, "unknown", lastChanged.Verdict)
	require.Equal(t, "configuration_changed", lastChanged.Result.Reason)
	// Snapshot JSON remains small: sample evidence is not included in the overview.
	targets, err := r.Store.Targets(ctx)
	require.NoError(t, err)
	require.NoError(t, r.Store.Enrich(ctx, targets, time.Now().Add(-time.Hour)))
	raw, err := json.Marshal(targets)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "Return ONLY")
}
