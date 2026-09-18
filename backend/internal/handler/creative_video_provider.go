package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type creativeVideoProviderCreateRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio"`
	Ratio       string `json:"ratio"`
	Resolution  string `json:"resolution"`
	Duration    int    `json:"duration"`
	Content     []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Role     string `json:"role"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	} `json:"content"`
	Image struct {
		URL string `json:"url"`
	} `json:"image"`
}

func (h *OpenAIGatewayHandler) CreativeVideoGeneration(c *gin.Context) {
	apiKey, subject, ok := h.creativeVideoSubject(c)
	if !ok {
		return
	}
	if h.creativeVideoService == nil || h.gatewayService == nil {
		batchImageError(c, service.ErrCreativeVideoDisabled)
		return
	}
	platform := service.NormalizeGroupPlatform(getAPIKeyGroupPlatform(apiKey))
	if platform != service.PlatformOpenAI && platform != service.PlatformMiniMax {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil || len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	var raw creativeVideoProviderCreateRequest
	if err := json.Unmarshal(body, &raw); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}
	req := service.CreativeVideoProviderRequest{
		Model:       strings.TrimSpace(raw.Model),
		Prompt:      strings.TrimSpace(raw.Prompt),
		AspectRatio: firstNonEmpty(strings.TrimSpace(raw.AspectRatio), strings.TrimSpace(raw.Ratio)),
		Resolution:  strings.TrimSpace(raw.Resolution),
		Duration:    raw.Duration,
		ImageURL:    strings.TrimSpace(raw.Image.URL),
	}
	if req.Prompt == "" || req.ImageURL == "" {
		for _, part := range raw.Content {
			switch strings.ToLower(strings.TrimSpace(part.Type)) {
			case "text":
				if req.Prompt == "" {
					req.Prompt = strings.TrimSpace(part.Text)
				}
			case "image_url":
				if req.ImageURL == "" {
					req.ImageURL = strings.TrimSpace(part.ImageURL.URL)
				}
			}
		}
	}
	if req.Model == "" || req.Prompt == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model and prompt are required")
		return
	}
	if decisionBody := []byte(req.Prompt); len(decisionBody) > 0 {
		decision := h.checkSecurityAudit(c, requestLogger(c, "handler.openai_gateway.creative_video"), apiKey, subject, service.ContentModerationProtocolOpenAIImages, req.Model, decisionBody)
		if decision != nil && !decision.AllowNextStage {
			h.openAISecurityAuditError(c, decision)
			return
		}
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}
	if err := h.creativeVideoService.CheckCreateAllowed(c.Request.Context(), subject.UserID); err != nil {
		batchImageError(c, err)
		return
	}
	selection, _, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
		service.WithOpenAIProfitControlSuppressed(c.Request.Context()),
		apiKey.GroupID,
		"",
		h.gatewayService.GenerateExplicitSessionHash(c, body),
		"",
		nil,
		service.OpenAIUpstreamTransportHTTPSSE,
		"",
		false,
		false,
		false,
		platform,
	)
	if err != nil || selection == nil || selection.Account == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "video_no_eligible_account", "No eligible video account")
		return
	}
	provider := service.NewCreativeVideoHTTPProvider(platform, nil)
	if !provider.SupportsAccount(selection.Account) {
		batchImageError(c, service.ErrCreativeVideoProviderUnsupportedAccount)
		return
	}
	task, err := h.creativeVideoService.CreateProviderPending(c.Request.Context(), service.BatchImageOwner{
		UserID:   subject.UserID,
		APIKeyID: apiKey.ID,
		GroupID:  apiKey.GroupID,
	}, provider.Name(), req)
	if err != nil {
		batchImageError(c, err)
		return
	}
	status, err := provider.Submit(c.Request.Context(), selection.Account, req)
	if err != nil {
		if task != nil {
			h.creativeVideoService.FailTask(c.Request.Context(), task.TaskID, "UPSTREAM_REQUEST_FAILED", err.Error())
		}
		logger.L().Warn("creative_video.provider_submit_failed",
			zap.String("provider", provider.Name()),
			zap.Int64("account_id", selection.Account.ID),
			zap.String("base_url", selection.Account.GetOpenAIBaseURL()),
			zap.String("model", req.Model),
			zap.Error(err),
		)
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video request failed")
		return
	}
	if task != nil {
		if err := h.creativeVideoService.CompleteProviderSubmit(c.Request.Context(), task.TaskID, selection.Account.ID, status, req); err != nil {
			h.creativeVideoService.FailTask(c.Request.Context(), task.TaskID, "TASK_SUBMIT_FAILED", err.Error())
			batchImageError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, service.CreativeVideoProviderResponseJSON(status))
}

func (h *OpenAIGatewayHandler) CreativeVideoStatus(c *gin.Context) {
	h.handleCreativeVideoLookup(c, false)
}

func (h *OpenAIGatewayHandler) CreativeVideoContent(c *gin.Context) {
	h.handleCreativeVideoLookup(c, true)
}

func (h *OpenAIGatewayHandler) handleCreativeVideoLookup(c *gin.Context, content bool) {
	apiKey, subject, ok := h.creativeVideoSubject(c)
	if !ok {
		return
	}
	platform := service.NormalizeGroupPlatform(getAPIKeyGroupPlatform(apiKey))
	if platform != service.PlatformOpenAI && platform != service.PlatformMiniMax {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	requestID := strings.TrimSpace(c.Param("request_id"))
	task, err := h.creativeVideoService.GetTask(c.Request.Context(), service.BatchImageOwner{UserID: subject.UserID, APIKeyID: apiKey.ID, GroupID: apiKey.GroupID}, requestID)
	if err != nil {
		batchImageError(c, err)
		return
	}
	if task.AccountID == nil || *task.AccountID <= 0 {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
		return
	}
	status := creativeVideoProviderStatusFromTask(task)
	var account *service.Account
	var provider *service.CreativeVideoHTTPProvider
	if creativeVideoTaskStatusNeedsSync(task.Status) || (content && status.Status == service.CreativeVideoStatusCompleted && platform == service.PlatformMiniMax && status.DownloadURL == "" && status.FileID == "") {
		account, err = h.gatewayService.GetAccountByID(c.Request.Context(), *task.AccountID)
		if err != nil || account == nil {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
		provider = service.NewCreativeVideoHTTPProvider(platform, nil)
		upstreamStatus, err := provider.Get(c.Request.Context(), account, requestID)
		if err != nil {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video status request failed")
			return
		}
		status = upstreamStatus
		fillCreativeVideoStatusFromTask(status, task)
		h.creativeVideoService.ObserveProviderTaskStatus(c.Request.Context(), service.BatchImageOwner{UserID: subject.UserID, APIKeyID: apiKey.ID, GroupID: apiKey.GroupID}, task.TaskID, requestID, status)
		h.recordCreativeVideoUsageIfCompleted(c, apiKey, subject, account, status, requestID, task.TaskID)
	}
	if !content {
		c.JSON(http.StatusOK, service.CreativeVideoProviderResponseJSON(status))
		return
	}
	if status.Status != service.CreativeVideoStatusCompleted {
		batchImageError(c, service.ErrCreativeVideoProviderOutputUnavailable)
		return
	}
	if account == nil {
		account, err = h.gatewayService.GetAccountByID(c.Request.Context(), *task.AccountID)
		if err != nil || account == nil {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
	}
	if provider == nil {
		provider = service.NewCreativeVideoHTTPProvider(platform, nil)
	}
	body, contentType, err := provider.OpenContent(c.Request.Context(), account, status)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video content request failed")
		return
	}
	defer func() { _ = body.Close() }()
	_ = h.creativeVideoService.MarkDownloaded(c.Request.Context(), service.BatchImageOwner{UserID: subject.UserID, APIKeyID: apiKey.ID, GroupID: apiKey.GroupID}, requestID)
	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	written, copyErr := io.Copy(c.Writer, body)
	_ = h.creativeVideoService.MarkOutputMetadata(c.Request.Context(), service.BatchImageOwner{UserID: subject.UserID, APIKeyID: apiKey.ID, GroupID: apiKey.GroupID}, requestID, written, contentType)
	if copyErr != nil {
		logger.L().Debug("creative_video.content_copy_failed", zap.String("request_id", requestID), zap.Error(copyErr))
	}
}

func creativeVideoProviderStatusFromTask(task *service.CreativeVideoTask) *service.CreativeVideoProviderStatus {
	if task == nil {
		return &service.CreativeVideoProviderStatus{}
	}
	status := &service.CreativeVideoProviderStatus{
		ID:              firstNonEmpty(creativeVideoTaskProviderRequestID(task), task.TaskID),
		Status:          task.Status,
		Model:           task.Model,
		DurationSeconds: 0,
	}
	if task.Resolution != nil {
		status.Resolution = *task.Resolution
	}
	if task.DurationSeconds != nil {
		status.DurationSeconds = *task.DurationSeconds
	}
	if task.DownloadURL != nil {
		status.DownloadURL = *task.DownloadURL
	}
	if task.FileID != nil {
		status.FileID = *task.FileID
	}
	return status
}

func fillCreativeVideoStatusFromTask(status *service.CreativeVideoProviderStatus, task *service.CreativeVideoTask) {
	if status == nil || task == nil {
		return
	}
	if strings.TrimSpace(status.Model) == "" {
		status.Model = task.Model
	}
	if strings.TrimSpace(status.Resolution) == "" && task.Resolution != nil {
		status.Resolution = *task.Resolution
	}
	if status.DurationSeconds <= 0 && task.DurationSeconds != nil {
		status.DurationSeconds = *task.DurationSeconds
	}
}

func (h *OpenAIGatewayHandler) creativeVideoSubject(c *gin.Context) (*service.APIKey, middleware2.AuthSubject, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return nil, middleware2.AuthSubject{}, false
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return nil, middleware2.AuthSubject{}, false
	}
	return apiKey, subject, true
}

func (h *OpenAIGatewayHandler) recordCreativeVideoUsageIfCompleted(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, account *service.Account, status *service.CreativeVideoProviderStatus, requestID, taskID string) {
	result := service.CreativeVideoForwardResultFromStatus(status, requestID)
	if result == nil {
		return
	}
	claimed, err := h.gatewayService.ClaimGrokVideoBilling(c.Request.Context(), requestID, subject.UserID, apiKey.ID)
	if err != nil || !claimed {
		return
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	sessionID := service.ExtractClientSessionID(c)
	payloadHash := service.HashUsageRequestPayload([]byte(requestID))
	h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    GetInboundEndpoint(c),
			UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: payloadHash,
			APIKeyService:      h.apiKeyService,
			QuotaPlatform:      service.QuotaPlatform(c.Request.Context(), apiKey),
			SessionID:          sessionID,
			ChannelUsageFields: service.ChannelUsageFields{
				OriginalModel:      status.Model,
				ChannelMappedModel: status.Model,
			},
			OnVideoUsageRecorded: func(actualCost float64) {
				_ = h.creativeVideoService.MarkUsageRecorded(ctx, service.BatchImageOwner{
					UserID:   subject.UserID,
					APIKeyID: apiKey.ID,
					GroupID:  apiKey.GroupID,
				}, firstNonEmpty(requestID, taskID), actualCost)
			},
		}); err != nil {
			_ = h.gatewayService.ReleaseGrokVideoBilling(ctx, requestID, subject.UserID, apiKey.ID)
			logger.L().With(zap.String("component", "handler.openai_gateway.creative_video")).Error("creative_video.record_usage_failed", zap.Error(err))
		}
	})
}

func getAPIKeyGroupPlatform(apiKey *service.APIKey) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}
