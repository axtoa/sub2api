//go:build integration

package quality

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityInventorySyncPreservesPreferences(t *testing.T) {
	db := qualityTestDB(t)
	s := &Store{DB: db}
	ctx := context.Background()
	cfg, err := s.Settings(ctx)
	require.NoError(t, err)
	p := cfg.Profile
	p.Questions = 4
	legacy, err := s.SaveTarget(ctx, Target{AccountID: 1, Model: "legacy-model", Enabled: false, Overrides: &p}, cfg)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO accounts(id,name,platform,type,credentials) VALUES
 (2,'mapped','anthropic','apikey','{"model_mapping":{"text-a":"claude-sonnet-4-6","text-b":"claude-opus-4-6","model-*":"claude-sonnet-4-6"}}'),
 (3,'unsupported','gemini','apikey','{}');
 INSERT INTO groups(id,name) VALUES(1,'group-a'),(2,'group-b');
 INSERT INTO account_groups(account_id,group_id) VALUES(1,1),(1,2);`)
	require.NoError(t, err)
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, e := s.SyncAccounts(ctx, true)
			require.NoError(t, e)
			mu.Lock()
			created += n
			mu.Unlock()
		}()
	}
	wg.Wait()
	require.Equal(t, 3, created)
	items, err := s.Inventory(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	byID := map[int64]MonitoredAccount{}
	for _, a := range items {
		byID[a.ID] = a
	}
	require.False(t, byID[1].Enabled, "all legacy target opt-outs are retained")
	require.Len(t, byID[1].Groups, 2)
	require.True(t, byID[3].Unsupported)
	current, err := s.Target(ctx, legacy.ID)
	require.NoError(t, err)
	require.False(t, current.Enabled)
	require.Equal(t, p, current.Effective(cfg))
	require.Equal(t, legacy.Version, current.Version)
	require.NoError(t, s.SaveAccount(ctx, 2, false, byID[2].Version))
	_, err = db.Exec(`INSERT INTO accounts(id,name,platform,type) VALUES(4,'new-account','openai','apikey')`)
	require.NoError(t, err)
	n, err := s.SyncAccounts(ctx, true)
	require.NoError(t, err)
	require.Zero(t, n)
	items, err = s.Inventory(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3, "refreshing results does not silently add new accounts")
	n, err = s.SyncAccounts(ctx, false)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	items, err = s.Inventory(ctx)
	require.NoError(t, err)
	require.Len(t, items, 4)
	for _, a := range items {
		if a.ID == 2 {
			require.False(t, a.Enabled)
			require.Equal(t, byID[2].Version+1, a.Version)
		}
	}
	targets, err := s.Targets(ctx)
	require.NoError(t, err)
	for _, target := range targets {
		require.NotEqual(t, "model-*", target.Model)
		if target.AccountID == 2 {
			require.False(t, target.AccountEnabled)
		}
	}
	_, err = db.Exec(`UPDATE accounts SET deleted_at=NOW() WHERE id=4`)
	require.NoError(t, err)
	items, err = s.Inventory(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
}

func TestQualityManualQueueIndependentAndBudgetExempt(t *testing.T) {
	db := qualityTestDB(t)
	ctx := context.Background()
	r := NewRunner(db, nil, nil, nil, "UTC")
	a := &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: "disabled", Schedulable: false, Concurrency: 1}
	r.Accounts = &testAccounts{a}
	r.Slots = &testSlots{acquired: true}
	prober := &testProber{mode: "availability"}
	r.Prober = prober
	cfg, err := r.Store.Settings(ctx)
	require.NoError(t, err)
	cfg.AutoEnabled = false
	cfg.DailyRequests = 1
	cfg, err = r.Store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	target, err := r.Store.SaveTarget(ctx, Target{AccountID: 1, Model: "text-model", Enabled: false}, cfg)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO quality_monitor_accounts(account_id,enabled) VALUES(1,FALSE)`)
	require.NoError(t, err)
	before, err := r.Store.Target(ctx, target.ID)
	require.NoError(t, err)
	day := time.Now().UTC().Format("2006-01-02")
	ok, err := r.Store.Reserve(ctx, day, 1000)
	require.NoError(t, err)
	require.True(t, ok)
	var wg sync.WaitGroup
	ids := sync.Map{}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job, e := r.Store.Queue(ctx, target.ID, "availability", true)
			require.NoError(t, e)
			_, duplicate := ids.LoadOrStore(job.ID, true)
			require.False(t, duplicate)
		}()
	}
	wg.Wait()
	_, err = r.Store.Queue(ctx, target.ID, "availability", false)
	require.True(t, isConflict(err), "auto jobs cannot multiply an existing queue")
	for i := 0; i < 12; i++ {
		job, e := r.Store.Claim(ctx)
		require.NoError(t, e)
		require.NotNil(t, job)
		other, e := r.Store.Claim(ctx)
		require.NoError(t, e)
		require.Nil(t, other, "a second worker must not execute the same target concurrently")
		require.NoError(t, r.execute(ctx, *job, cfg))
	}
	require.Equal(t, 12, prober.calls)
	history, err := r.Store.History(ctx, target.ID, 60)
	require.NoError(t, err)
	require.Len(t, history, 12)
	for _, job := range history {
		require.Equal(t, "passed", job.Verdict)
		require.True(t, job.Manual)
	}
	budget, err := r.Store.Budget(ctx, cfg, time.Now(), time.UTC)
	require.NoError(t, err)
	require.Equal(t, 1, budget.Requests)
	require.Equal(t, 12, budget.ManualRequests)
	require.EqualValues(t, 360, budget.ManualTokens)
	after, err := r.Store.Target(ctx, target.ID)
	require.NoError(t, err)
	require.WithinDuration(t, before.NextAvailabilityAt, after.NextAvailabilityAt, time.Millisecond)
	require.False(t, after.Enabled)
	require.False(t, after.AccountEnabled)
	require.NoError(t, r.Store.Enrich(ctx, []Target{after}, time.Now().Add(-time.Hour)))
}
