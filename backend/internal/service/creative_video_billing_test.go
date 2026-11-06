package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreativeVideoSettlementRetryUsesStableIdentity(t *testing.T) {
	usage := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usage, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	billing := &openAIRecordUsageBillingRepoStub{}
	svc.usageBillingRepo = billing
	requestID := "upstream-video-123"
	groupID := int64(123)
	price := 0.04
	key := &APIKey{ID: 1, UserID: 2, User: &User{ID: 2}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformGrok, RateMultiplier: 1, VideoRateIndependent: true, VideoRateMultiplier: 1, VideoPrice720P: &price}}
	account := &Account{ID: 3, Platform: PlatformGrok}
	task := &CreativeVideoTask{TaskID: "vidtask_123", UserID: 2, APIKeyID: 1, GroupID: &groupID, Provider: PlatformGrok, ProviderRequestID: &requestID, Model: "grok-imagine-video", CreatedAt: time.Now()}
	status := &CreativeVideoProviderStatus{ID: requestID, Status: CreativeVideoStatusCompleted, Model: "upstream-alias", Resolution: "720p", DurationSeconds: 8}
	failedMark := errors.New("task update failed")
	err := svc.RecordCreativeVideoTaskUsage(context.Background(), key, account, task, status, nil, func(float64) error { return failedMark })
	require.ErrorIs(t, err, failedMark)
	first := *billing.lastCmd
	// Retry after the charge committed but the task record update failed.
	billing.result = &UsageBillingApplyResult{Applied: false}
	marked := false
	err = svc.RecordCreativeVideoTaskUsage(context.Background(), key, account, task, status, nil, func(cost float64) error { marked = true; require.InDelta(t, 0.32, cost, 1e-9); return nil })
	require.NoError(t, err)
	require.True(t, marked)
	require.Equal(t, first.RequestID, billing.lastCmd.RequestID)
	require.Equal(t, StableGrokVideoBillingRequestID(requestID), billing.lastCmd.RequestID)
	require.Equal(t, "grok-imagine-video", usage.lastLog.Model)
}

func TestCreativeVideoSettlementRejectsChangedBillingGroup(t *testing.T) {
	oldGroup, newGroup := int64(1), int64(2)
	requestID := "task-1"
	svc := &OpenAIGatewayService{}
	err := svc.RecordCreativeVideoTaskUsage(context.Background(), &APIKey{User: &User{}, GroupID: &newGroup}, &Account{}, &CreativeVideoTask{GroupID: &oldGroup, ProviderRequestID: &requestID}, &CreativeVideoProviderStatus{Status: CreativeVideoStatusCompleted}, nil, func(float64) error { t.Fatal("must not bill another group"); return nil })
	require.ErrorContains(t, err, "original group unavailable")
}

func TestCreativeVideoSettlementUsesTaskOriginalGroupAfterKeyMoves(t *testing.T) {
	oldGroup, newGroup := int64(123), int64(456)
	price := 0.04
	requestID := "upstream-video-moved"
	usage := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usage, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.usageBillingRepo = &openAIRecordUsageBillingRepoStub{}
	svc.channelService = &ChannelService{groupRepo: &openAIMediaPriceGroupRepoStub{group: &Group{ID: oldGroup, Platform: PlatformGrok, RateMultiplier: 1, VideoRateIndependent: true, VideoRateMultiplier: 1, VideoPrice720P: &price}}}
	channelCache := newEmptyChannelCache()
	channelCache.loadedAt = time.Now()
	svc.channelService.cache.Store(channelCache)
	key := &APIKey{ID: 1, UserID: 2, User: &User{ID: 2}, GroupID: &newGroup, Group: &Group{ID: newGroup, Platform: PlatformGrok}}
	task := &CreativeVideoTask{UserID: 2, APIKeyID: 1, GroupID: &oldGroup, Provider: PlatformGrok, ProviderRequestID: &requestID, Model: "grok-imagine-video", CreatedAt: time.Now()}
	status := &CreativeVideoProviderStatus{ID: requestID, Status: CreativeVideoStatusCompleted, Resolution: "720p", DurationSeconds: 8}
	var cost float64
	err := svc.RecordCreativeVideoTaskUsage(context.Background(), key, &Account{ID: 3, Platform: PlatformGrok}, task, status, nil, func(value float64) error { cost = value; return nil })
	require.NoError(t, err)
	require.InDelta(t, 0.32, cost, 1e-9)
	require.Equal(t, &newGroup, key.GroupID, "settlement must not mutate the current key")
}
