//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Exercise the production billing entry points, including quota commands, rather
// than only the arithmetic helpers. Upstream results must remain reusable as-is.
func TestRecordUsageTokenMultiplier(t *testing.T) {
	for _, engine := range []string{"gateway", "openai"} {
		for _, mode := range []string{"balance", "subscription", "free", "simple"} {
			for _, stream := range []bool{false, true} {
				t.Run(engine+"/"+mode+map[bool]string{true: "/stream", false: "/sync"}[stream], func(t *testing.T) {
					record := func(multiplier float64) (*UsageLog, *UsageBillingCommand) {
						logs := &openAIRecordUsageLogRepoStub{}
						billing := &openAIRecordUsageBillingRepoStub{}
						group := &Group{RateMultiplier: 0.8, TokenUsageMultiplier: multiplier}
						var subscription *UserSubscription
						if mode == "subscription" {
							group.SubscriptionType = SubscriptionTypeSubscription
							subscription = &UserSubscription{ID: 900}
						}
						if mode == "free" {
							group.RateMultiplier = 0
						}
						groupID := int64(801)
						key := &APIKey{ID: 501, Quota: 100, RateLimit5h: 100, Group: group, GroupID: &groupID}
						user := &User{ID: 601}
						account := &Account{ID: 701, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": float64(100)}}
						quota := &openAIRecordUsageAPIKeyQuotaStub{}
						if engine == "gateway" {
							svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
							if mode == "simple" {
								svc.cfg.RunMode = config.RunModeSimple
							}
							result := &ForwardResult{RequestID: "multiplier-test", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 101, OutputTokens: 31}, Duration: time.Second, Stream: stream}
							require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: key, User: user, Account: account, Subscription: subscription, APIKeyService: quota}))
							require.Equal(t, 101, result.Usage.InputTokens)
							require.Equal(t, 31, result.Usage.OutputTokens)
						} else {
							svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
							if mode == "simple" {
								svc.cfg.RunMode = config.RunModeSimple
							}
							result := &OpenAIForwardResult{RequestID: "multiplier-test", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 101, OutputTokens: 31}, Duration: time.Second, Stream: stream}
							require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: user, Account: account, Subscription: subscription, APIKeyService: quota}))
							require.Equal(t, 101, result.Usage.InputTokens)
							require.Equal(t, 31, result.Usage.OutputTokens)
						}
						require.NotNil(t, logs.lastLog)
						return logs.lastLog, billing.lastCmd
					}
					baseLog, baseCmd := record(1)
					log, cmd := record(1.5)
					require.Equal(t, baseLog.TotalCost, log.TotalCost)
					require.Equal(t, 101, log.ActualInputTokens)
					require.Equal(t, 31, log.ActualOutputTokens)
					if mode == "simple" {
						require.Nil(t, cmd)
						require.Equal(t, 101, log.InputTokens)
						require.Equal(t, baseLog.ActualCost, log.ActualCost)
						return
					}
					require.Equal(t, 152, log.InputTokens)
					require.Equal(t, 47, log.OutputTokens)
					require.NotNil(t, cmd)
					require.InDelta(t, baseLog.ActualCost*1.5, log.ActualCost, 1e-8)
					require.Equal(t, baseCmd.AccountQuotaCost, cmd.AccountQuotaCost)
					require.Greater(t, cmd.AccountQuotaCost, 0.0)
					require.InDelta(t, baseCmd.BalanceCost*1.5, cmd.BalanceCost, 1e-8)
					require.InDelta(t, baseCmd.SubscriptionCost*1.5, cmd.SubscriptionCost, 1e-8)
					require.InDelta(t, baseCmd.APIKeyQuotaCost*1.5, cmd.APIKeyQuotaCost, 1e-8)
					require.InDelta(t, baseCmd.APIKeyRateLimitCost*1.5, cmd.APIKeyRateLimitCost, 1e-8)
					if mode == "free" {
						require.Zero(t, log.ActualCost)
					} else {
						require.Greater(t, log.ActualCost, 0.0)
					}
				})
			}
		}
	}
}

func TestBatchImageTokenMultiplierSnapshot(t *testing.T) {
	svc, repo, _, _, _ := newTestBatchImagePublicService(true)
	groupID := int64(7)
	group := &Group{ID: groupID, Platform: PlatformGemini, RateMultiplier: 1, TokenUsageMultiplier: 2, AllowImageGeneration: true, AllowBatchImageGeneration: true, BatchImageDiscountMultiplier: 0.5, BatchImageHoldMultiplier: 0.6}
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: map[int64]*Group{groupID: group}}
	batch, err := svc.Submit(context.Background(), BatchImageOwner{UserID: 11, APIKeyID: 22, GroupID: &groupID}, validBatchImageSubmitRequest(), "")
	require.NoError(t, err)
	job := repo.jobs[batch.ID]
	require.Equal(t, 2.0, job.TokenUsageMultiplier)
	require.InDelta(t, 0.6, *job.HoldAmount, 1e-12)
	require.InDelta(t, 0.5, job.EstimatedCost, 1e-12)
	// Change the live group before settlement: the submitted price stays fixed.
	group.TokenUsageMultiplier = 5
	job.Status = BatchImageJobStatusSettling
	job.SuccessCount, job.FailCount = 1, 1
	billing := &fakeBatchImageBillingRepo{}
	logs := &openAIRecordUsageLogRepoStub{}
	settlement := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: logs, Pricing: &fakeBatchImagePricingResolver{unitPrice: 99}}
	result, err := settlement.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.InDelta(t, 0.25, result.ActualCost, 1e-12)
	require.InDelta(t, 0.125, logs.lastLog.TotalCost, 1e-12)
	require.Equal(t, 2.0, logs.lastLog.TokenUsageMultiplier)
	require.InDelta(t, 0.6, billing.captures[0].HoldAmount, 1e-12)
	require.InDelta(t, 0.25, billing.captures[0].ActualAmount, 1e-12)
	_, err = settlement.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Len(t, billing.captures, 1, "settlement retry must not charge twice")
}
