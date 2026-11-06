package service

import (
	"context"
	"fmt"
	"strings"
)

// RecordUsage supplies durable task-ID deduplication; Redis claims used by
// interactive polling must not prevent a worker retry after a process crash.
func (s *OpenAIGatewayService) RecordCreativeVideoTaskUsage(ctx context.Context, key *APIKey, account *Account, task *CreativeVideoTask, status *CreativeVideoProviderStatus, keys *APIKeyService, mark func(float64) error) error {
	if task == nil || task.ProviderRequestID == nil || strings.TrimSpace(*task.ProviderRequestID) == "" || key == nil || key.User == nil || account == nil || mark == nil {
		return fmt.Errorf("video billing context unavailable")
	}
	if task.UserID != key.UserID || task.APIKeyID != key.ID || key.User.ID != key.UserID {
		return fmt.Errorf("video billing owner mismatch")
	}
	if (task.GroupID == nil) != (key.GroupID == nil) || (task.GroupID != nil && *key.GroupID != *task.GroupID) {
		if task.GroupID == nil || s.channelService == nil || s.channelService.groupRepo == nil {
			return fmt.Errorf("video billing original group unavailable")
		}
		group, err := s.channelService.groupRepo.GetByIDLite(ctx, *task.GroupID)
		if err != nil {
			return fmt.Errorf("video billing original group unavailable: %w", err)
		}
		if group == nil || group.ID != *task.GroupID {
			return fmt.Errorf("video billing original group unavailable")
		}
		originalKey := *key
		originalKey.GroupID = task.GroupID
		originalKey.Group = group
		key = &originalKey
	}
	result := CreativeVideoForwardResultFromStatus(status, *task.ProviderRequestID)
	if result == nil {
		return nil
	}
	// Keep the public model for pricing; upstream aliases are attribution only.
	if strings.TrimSpace(task.Model) != "" {
		result.Model = task.Model
		result.BillingModel = task.Model
	}
	result.UpstreamModel = status.Model
	var subscription *UserSubscription
	var err error
	if key.Group != nil && key.Group.IsSubscriptionType() {
		if s.userSubRepo == nil {
			return fmt.Errorf("video billing subscription repository unavailable")
		}
		subscription, err = s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, key.UserID, key.Group.ID)
		if err != nil {
			return err
		}
	}
	var markErr error
	err = s.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: result, PricingAt: task.CreatedAt, APIKey: key, User: key.User, Account: account, Subscription: subscription,
		APIKeyService: keys, RequestPayloadHash: HashUsageRequestPayload([]byte(*task.ProviderRequestID)),
		InboundEndpoint: "/v1/videos/generations", UpstreamEndpoint: "/v1/videos", QuotaPlatform: task.Provider,
		ChannelUsageFields:   ChannelUsageFields{OriginalModel: task.Model, ChannelMappedModel: status.Model},
		OnVideoUsageRecorded: func(cost float64) { markErr = mark(cost) },
	})
	if err != nil {
		return err
	}
	return markErr
}
