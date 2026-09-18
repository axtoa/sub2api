package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) CreativeVideoTasks(c *gin.Context) {
	owner, ok := batchImageOwnerFromContext(c)
	if !ok {
		batchImageError(c, infraerrors.New(http.StatusUnauthorized, "API_KEY_REQUIRED", "API key is required"))
		return
	}
	if h == nil || h.creativeVideoService == nil {
		c.JSON(http.StatusOK, service.CreativeVideoTasksResponse{
			Object:            "list",
			Data:              []service.CreativeVideoTaskPublic{},
			RetentionDays:     service.CreativeWorkbenchRetentionDaysDefault,
			MaxRecordsPerUser: service.CreativeWorkbenchMaxRecordsDefault,
			MaxRunningPerUser: service.CreativeWorkbenchVideoRunningDefault,
		})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	query := service.CreativeVideoTasksQuery{
		Status: c.Query("status"),
		Limit:  limit,
		Cursor: c.Query("cursor"),
	}
	records, err := h.creativeVideoService.ListRecords(c.Request.Context(), owner, query)
	if err != nil {
		batchImageError(c, err)
		return
	}
	if h.syncCreativeVideoListStatuses(c, owner, records.Tasks) {
		records, err = h.creativeVideoService.ListRecords(c.Request.Context(), owner, query)
		if err != nil {
			batchImageError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, creativeVideoTasksResponseFromRecords(records))
}

func (h *OpenAIGatewayHandler) DeleteCreativeVideoTask(c *gin.Context) {
	owner, ok := batchImageOwnerFromContext(c)
	if !ok {
		batchImageError(c, infraerrors.New(http.StatusUnauthorized, "API_KEY_REQUIRED", "API key is required"))
		return
	}
	if h == nil || h.creativeVideoService == nil {
		c.Status(http.StatusNoContent)
		return
	}
	if err := h.creativeVideoService.DeleteRecord(c.Request.Context(), owner, c.Param("request_id")); err != nil {
		batchImageError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func creativeVideoTasksResponseFromRecords(records *service.CreativeVideoTaskListRecords) service.CreativeVideoTasksResponse {
	settings := service.DefaultCreativeWorkbenchSettings()
	if records != nil && records.Settings != nil {
		settings = records.Settings
	}
	data := []service.CreativeVideoTaskPublic{}
	hasMore := false
	if records != nil {
		hasMore = records.HasMore
		data = make([]service.CreativeVideoTaskPublic, 0, len(records.Tasks))
		for _, task := range records.Tasks {
			data = append(data, service.CreativeVideoTaskToPublic(task))
		}
	}
	return service.CreativeVideoTasksResponse{
		Object:            "list",
		Data:              data,
		HasMore:           hasMore,
		RetentionDays:     settings.RetentionDays,
		MaxRecordsPerUser: settings.MaxRecordsPerUser,
		MaxRunningPerUser: settings.VideoMaxRunningPerUser,
	}
}

func (h *OpenAIGatewayHandler) syncCreativeVideoListStatuses(c *gin.Context, owner service.BatchImageOwner, tasks []*service.CreativeVideoTask) bool {
	if h == nil || h.gatewayService == nil || h.creativeVideoService == nil || len(tasks) == 0 {
		return false
	}
	apiKey, apiKeyOK := middleware2.GetAPIKeyFromContext(c)
	subject, subjectOK := middleware2.GetAuthSubjectFromContext(c)
	updated := false
	checked := 0
	const maxChecks = 5
	for _, task := range tasks {
		if task == nil || !creativeVideoTaskStatusNeedsSync(task.Status) || task.AccountID == nil || *task.AccountID <= 0 {
			continue
		}
		if !task.UpdatedAt.IsZero() && time.Since(task.UpdatedAt) < 30*time.Second {
			continue
		}
		requestID := creativeVideoTaskProviderRequestID(task)
		if requestID == "" {
			continue
		}
		platform := service.NormalizeGroupPlatform(task.Provider)
		if platform != service.PlatformOpenAI && platform != service.PlatformMiniMax {
			continue
		}
		checked++
		if checked > maxChecks {
			break
		}
		account, err := h.gatewayService.GetAccountByID(c.Request.Context(), *task.AccountID)
		if err != nil || account == nil {
			continue
		}
		provider := service.NewCreativeVideoHTTPProvider(platform, nil)
		if !provider.SupportsAccount(account) {
			continue
		}
		syncCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		status, err := provider.Get(syncCtx, account, requestID)
		cancel()
		if err != nil {
			logger.L().Debug("creative_video.list_status_sync_failed",
				zap.String("provider", provider.Name()),
				zap.Int64("account_id", account.ID),
				zap.String("request_id", requestID),
				zap.Error(err),
			)
			continue
		}
		fillCreativeVideoStatusFromTask(status, task)
		h.creativeVideoService.ObserveProviderTaskStatus(c.Request.Context(), owner, task.TaskID, requestID, status)
		if status != nil && status.Status != "" && status.Status != task.Status {
			updated = true
		}
		if apiKeyOK && subjectOK && status != nil && status.Status == service.CreativeVideoStatusCompleted {
			h.recordCreativeVideoUsageIfCompleted(c, apiKey, subject, account, status, requestID, task.TaskID)
		}
	}
	return updated
}

func creativeVideoTaskStatusNeedsSync(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case service.CreativeVideoStatusQueued, service.CreativeVideoStatusSubmitted, service.CreativeVideoStatusRunning:
		return true
	default:
		return false
	}
}

func creativeVideoTaskProviderRequestID(task *service.CreativeVideoTask) string {
	if task == nil || task.ProviderRequestID == nil {
		return ""
	}
	return strings.TrimSpace(*task.ProviderRequestID)
}
