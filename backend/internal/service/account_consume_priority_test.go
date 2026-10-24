//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsumePriorityDefaultsAndDisabledOrder(t *testing.T) {
	for _, value := range []any{nil, false, "true", 1, map[string]any{"enabled": true}} {
		account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{ConsumePriorityExtraKey: value}}
		require.False(t, account.ConsumePriorityEnabled())
	}
	require.False(t, (*Account)(nil).ConsumePriorityEnabled())
	require.False(t, (&Account{Platform: PlatformGemini, Extra: map[string]any{ConsumePriorityExtraKey: true}}).ConsumePriorityEnabled())
	accounts := []accountWithLoad{{account: &Account{ID: 1, Platform: PlatformAnthropic}}, {account: &Account{ID: 2, Platform: PlatformAnthropic}}}
	require.Same(t, &accounts[0], &preferredAvailableAccounts(accounts)[0])
	require.Same(t, &accounts[0], &preferConsumptionInOrder(accounts)[0])
	plan := openAIAccountLoadPlan{topK: 1, selectionOrder: []openAIAccountCandidateScore{{account: &Account{ID: 1, Platform: PlatformOpenAI}}}}
	order, id := consumePriorityOrder(OpenAIAccountScheduleRequest{}, plan)
	require.Zero(t, id)
	require.Same(t, &plan.selectionOrder[0], &order[0], "disabled path must return the original order without ranking/random draws")
}

func TestConsumePriorityOpenAISelection(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, scenario := range []string{"missing", "off", "on", "busy", "slot_race", "disabled", "limited", "expired", "quota", "other_group", "model_mismatch", "excluded", "sticky"} {
			t.Run(map[bool]string{false: "legacy/", true: "advanced/"}[advanced]+scenario, func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
				accounts := []Account{
					{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, Priority: 0},
					{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, Priority: 100, Extra: map[string]any{ConsumePriorityExtraKey: true}},
				}
				loads := map[int64]*AccountLoadInfo{}
				acquires := map[int64]bool{}
				var excluded map[int64]struct{}
				session := ""
				cache := &schedulerTestGatewayCache{}
				switch scenario {
				case "missing":
					accounts[1].Extra = nil
				case "off":
					accounts[1].Extra[ConsumePriorityExtraKey] = false
				case "busy":
					loads[2] = &AccountLoadInfo{AccountID: 2, LoadRate: 100, CurrentConcurrency: 2}
				case "slot_race":
					acquires[2] = false
				case "disabled":
					accounts[1].Schedulable = false
				case "limited":
					until := time.Now().Add(time.Hour)
					accounts[1].RateLimitResetAt = &until
				case "expired":
					past := time.Now().Add(-time.Hour)
					accounts[1].ExpiresAt = &past
					accounts[1].AutoPauseOnExpired = true
				case "quota":
					accounts[1].Extra["quota_limit"] = 1.0
					accounts[1].Extra["quota_used"] = 1.0
				case "other_group":
					accounts[1].GroupIDs = []int64{99}
				case "model_mismatch":
					accounts[1].Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
				case "excluded":
					excluded = map[int64]struct{}{2: {}}
				case "sticky":
					session = "existing-session"
				}
				var acquired, released []int64
				cfg := newSchedulerTestSubscriptionPriorityConfig()
				cfg.Gateway.Scheduling.LoadBatchEnabled = true
				svc := &OpenAIGatewayService{cfg: cfg, accountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}}, cache: cache,
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{loadMap: loads, acquireResults: acquires, acquiredIDs: &acquired, releasedIDs: &released})}
				if session != "" {
					require.NoError(t, svc.setStickySessionAccountID(context.Background(), nil, session, 1, time.Hour))
				}
				var result *AccountSelectionResult
				var err error
				if advanced {
					scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
					result, _, err = scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-5.1", SessionHash: session, ExcludedIDs: excluded})
				} else {
					result, err = svc.selectAccountWithLoadAwareness(context.Background(), nil, PlatformOpenAI, session, "gpt-5.1", excluded, false, "", false)
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.True(t, result.Acquired)
				wantID, reason := int64(1), AccountSelectionNormal
				if scenario == "on" {
					wantID, reason = 2, AccountSelectionPriority
				}
				if scenario == "busy" || scenario == "slot_race" {
					reason = AccountSelectionFallback
				}
				require.Equal(t, wantID, result.Account.ID)
				require.Equal(t, reason, result.AccountSelectionReason)
				require.Equal(t, 100, accounts[1].Priority, "numeric priority must not be overwritten")
				result.ReleaseFunc()
				require.Contains(t, released, wantID)
			})
		}
	}
}

func TestConsumePriorityAnthropicSelection(t *testing.T) {
	for _, scenario := range []string{"off", "on", "busy", "slot_race", "excluded", "sticky"} {
		t.Run(scenario, func(t *testing.T) {
			repo := &mockAccountRepoForPlatform{accounts: []Account{
				{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Priority: 0, Status: StatusActive, Schedulable: true, Concurrency: 2},
				{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Priority: 100, Status: StatusActive, Schedulable: true, Concurrency: 2, Extra: map[string]any{ConsumePriorityExtraKey: scenario != "off"}},
			}, accountsByID: map[int64]*Account{}}
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			loads, acquires := map[int64]*AccountLoadInfo{}, map[int64]bool{}
			var excluded map[int64]struct{}
			cache := &mockGatewayCacheForPlatform{}
			session := ""
			switch scenario {
			case "busy":
				loads[2] = &AccountLoadInfo{AccountID: 2, LoadRate: 100, CurrentConcurrency: 2}
			case "slot_race":
				acquires[2] = false
			case "excluded":
				excluded = map[int64]struct{}{2: {}}
			case "sticky":
				session = "existing"
				cache.sessionBindings = map[string]int64{session: 1}
			}
			cfg := testConfig()
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			svc := &GatewayService{cfg: cfg, accountRepo: repo, cache: cache, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{loadMap: loads, acquireResults: acquires})}
			result, err := svc.SelectAccountWithLoadAwareness(context.Background(), nil, session, "claude-3-5-sonnet-20241022", excluded, "", 0)
			require.NoError(t, err)
			require.NotNil(t, result)
			wantID, reason := int64(1), AccountSelectionNormal
			if scenario == "on" {
				wantID, reason = 2, AccountSelectionPriority
			}
			if scenario == "busy" || scenario == "slot_race" {
				reason = AccountSelectionFallback
			}
			require.Equal(t, wantID, result.Account.ID)
			require.Equal(t, reason, result.AccountSelectionReason)
			result.ReleaseFunc()
		})
	}
}

type consumePriorityErrorCache struct{ schedulerTestConcurrencyCache }

func (c consumePriorityErrorCache) AcquireAccountSlot(ctx context.Context, id int64, limit int, requestID string) (bool, error) {
	if id == 2 {
		return false, errors.New("preferred slot unavailable")
	}
	return c.schedulerTestConcurrencyCache.AcquireAccountSlot(ctx, id, limit, requestID)
}

func TestConsumePriorityOptionalErrorKeepsOrdinaryBudgetAndFallback(t *testing.T) {
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, Extra: map[string]any{ConsumePriorityExtraKey: true}},
	}
	candidates := []openAIAccountCandidateScore{
		{account: &accounts[0], loadInfo: &AccountLoadInfo{}, score: 1},
		{account: &accounts[1], loadInfo: &AccountLoadInfo{}, score: 0},
	}
	svc := &OpenAIGatewayService{cfg: testConfig(), accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, concurrencyService: NewConcurrencyService(consumePriorityErrorCache{})}
	s := &defaultOpenAIAccountScheduler{service: svc}
	budget := newOpenAISelectionProbeBudget()
	budget.enableLimit()
	plan := openAIAccountLoadPlan{topK: 1, candidates: candidates, selectionOrder: candidates[:1]}
	result, _, attempted, err := s.tryAcquireWithConsumePriority(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-5.1"}, plan, budget)
	require.NoError(t, err)
	require.True(t, attempted)
	require.Equal(t, int64(1), result.Account.ID)
	require.Equal(t, AccountSelectionFallback, result.AccountSelectionReason)
	result.ReleaseFunc()
	require.Equal(t, int64(1), plan.selectionOrder[0].account.ID, "waiting order remains ordinary")
}

func TestConsumePriorityPreservesSpecialPoliciesAndAttribution(t *testing.T) {
	a := &Account{ID: 2, Platform: PlatformOpenAI, Extra: map[string]any{ConsumePriorityExtraKey: true}}
	plan := openAIAccountLoadPlan{topK: 1, candidates: []openAIAccountCandidateScore{{account: a}}, selectionOrder: []openAIAccountCandidateScore{{account: &Account{ID: 1}}}}
	for _, req := range []OpenAIAccountScheduleRequest{
		{RequireCompact: true}, {RequiredCapability: OpenAIEndpointCapabilityEmbeddings},
		{RequiredImageCapability: OpenAIImagesCapabilityBasic}, {StickyWeighted: true, StickyAccountID: 1}, {Platform: PlatformGrok},
	} {
		order, id := consumePriorityOrder(req, plan)
		require.Zero(t, id)
		require.Same(t, &plan.selectionOrder[0], &order[0])
	}
	require.Equal(t, AccountSelectionFallback, AccountSelectionReasonAfterRetry(AccountSelectionPriority, AccountSelectionNormal))
	require.Equal(t, AccountSelectionPriority, AccountSelectionReasonAfterRetry(AccountSelectionFallback, AccountSelectionPriority))
	require.Equal(t, AccountSelectionNormal, AccountSelectionReasonForTurn(AccountSelectionPriority, 2))
	require.Nil(t, accountSelectionReasonPointer(""))
	require.Nil(t, accountSelectionReasonPointer("unknown"))
}

func TestConsumePriorityPreservesSubscriptionPoolAndFallbackReason(t *testing.T) {
	for _, busy := range []bool{false, true} {
		accounts := []Account{
			{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"plan_type": "plus"}},
			{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}
		preferred := 1 // the regular pool preference must not displace the explicit subscription policy
		if busy {
			preferred = 0
		}
		accounts[preferred].Extra = map[string]any{ConsumePriorityExtraKey: true}
		cache := schedulerTestConcurrencyCache{acquireResults: map[int64]bool{1: !busy, 2: true}}
		svc := &OpenAIGatewayService{cfg: newSchedulerTestSubscriptionPriorityConfig(), accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, concurrencyService: NewConcurrencyService(cache)}
		s := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
		result, _, err := s.Select(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-5.1", SubscriptionPriority: true})
		require.NoError(t, err)
		if busy {
			require.Equal(t, int64(2), result.Account.ID)
			require.Equal(t, AccountSelectionFallback, result.AccountSelectionReason)
		} else {
			require.Equal(t, int64(1), result.Account.ID)
			require.Equal(t, AccountSelectionNormal, result.AccountSelectionReason)
		}
		result.ReleaseFunc()
	}
}
