package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// Claiming in PostgreSQL permits several application instances to share the
// worker. Completed but unbilled tasks remain eligible after a process restart.
func (h *OpenAIGatewayHandler) reconcileCreativeVideoTasks(ctx context.Context) error {
	repo, ok := h.creativeVideoService.Repo.(interface {
		ClaimCreativeVideoTasksForSync(context.Context, int) ([]*service.CreativeVideoTask, error)
	})
	if !ok || h.gatewayService == nil || h.apiKeyService == nil {
		return nil
	}
	tasks, err := repo.ClaimCreativeVideoTasksForSync(ctx, 4)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		itemCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := h.reconcileCreativeVideoTask(itemCtx, task)
		cancel()
		if err != nil {
			logger.L().Warn("creative_video.task_reconcile_failed", zap.String("task_id", task.TaskID), zap.Error(err))
		}
	}
	return nil
}

func (h *OpenAIGatewayHandler) reconcileCreativeVideoTask(ctx context.Context, task *service.CreativeVideoTask) error {
	if task.AccountID == nil || task.ProviderRequestID == nil {
		return nil
	}
	account, err := h.gatewayService.GetAccountByID(ctx, *task.AccountID)
	if err != nil {
		return err
	}
	owner := service.BatchImageOwner{UserID: task.UserID, APIKeyID: task.APIKeyID, GroupID: task.GroupID}
	status := creativeVideoProviderStatusFromTask(task)
	// Deleting a completed record hides its output, but does not cancel its bill.
	if task.Status == service.CreativeVideoStatusOutputDeleted {
		status.Status = service.CreativeVideoStatusCompleted
	}
	if status.Status != service.CreativeVideoStatusCompleted {
		if task.Provider == service.CreativeVideoProviderGrok {
			status, err = h.gatewayService.GetCreativeGrokVideoStatus(ctx, account, *task.ProviderRequestID)
		} else {
			status, err = service.NewCreativeVideoHTTPProvider(task.Provider, nil).Get(ctx, account, *task.ProviderRequestID)
		}
		if err != nil {
			return err
		}
		fillCreativeVideoStatusFromTask(status, task)
		h.creativeVideoService.ObserveProviderTaskStatus(ctx, owner, task.TaskID, *task.ProviderRequestID, status)
	}
	if status == nil || status.Status != service.CreativeVideoStatusCompleted || task.ActualCost != nil {
		return nil
	}
	key, err := h.apiKeyService.GetByID(ctx, task.APIKeyID)
	if err != nil {
		return err
	}
	if key == nil || key.User == nil || key.UserID != task.UserID {
		return fmt.Errorf("video billing owner unavailable")
	}
	return h.gatewayService.RecordCreativeVideoTaskUsage(ctx, key, account, task, status, h.apiKeyService,
		func(cost float64) error {
			return h.creativeVideoService.MarkUsageRecorded(ctx, owner, task.TaskID, cost)
		})
}
