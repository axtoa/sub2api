package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	CreativeVideoProviderGrok = "grok"

	CreativeVideoStatusQueued        = "queued"
	CreativeVideoStatusSubmitted     = "submitted"
	CreativeVideoStatusRunning       = "running"
	CreativeVideoStatusCompleted     = "completed"
	CreativeVideoStatusFailed        = "failed"
	CreativeVideoStatusExpired       = "expired"
	CreativeVideoStatusOutputDeleted = "output_deleted"

	CreativeWorkbenchHeader      = "X-Sub2API-Creative-Workbench"
	CreativeWorkbenchHeaderVideo = "video"
)

var (
	ErrCreativeVideoDisabled             = infraerrors.New(http.StatusNotFound, "CREATIVE_VIDEO_DISABLED", "creative video is disabled")
	ErrCreativeVideoRunningLimitExceeded = infraerrors.New(http.StatusTooManyRequests, "CREATIVE_VIDEO_RUNNING_LIMIT_EXCEEDED", "too many running creative video tasks")
	ErrCreativeVideoTaskNotFound         = infraerrors.New(http.StatusNotFound, "CREATIVE_VIDEO_TASK_NOT_FOUND", "creative video task not found")
)

type CreativeVideoTask struct {
	ID                int64
	TaskID            string
	ProviderRequestID *string
	UserID            int64
	APIKeyID          int64
	GroupID           *int64
	AccountID         *int64
	Provider          string
	Model             string
	PromptPreview     *string
	Status            string
	Resolution        *string
	DurationSeconds   *int
	OutputExpiresAt   *time.Time
	DownloadedAt      *time.Time
	OutputDeletedAt   *time.Time
	UserDeletedAt     *time.Time
	LastErrorCode     *string
	LastErrorMessage  *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	SubmittedAt       *time.Time
	CompletedAt       *time.Time
}

type CreateCreativeVideoTaskParams struct {
	TaskID                string
	UserID                int64
	APIKeyID              int64
	GroupID               *int64
	Provider              string
	Model                 string
	PromptPreview         *string
	Status                string
	Resolution            *string
	DurationSeconds       *int
	OutputExpiresAt       *time.Time
	MaxActiveTasksPerUser int
}

type CompleteCreativeVideoTaskSubmitParams struct {
	TaskID            string
	ProviderRequestID string
	AccountID         int64
	Status            string
	Model             string
	Resolution        string
	DurationSeconds   int
}

type ObserveCreativeVideoTaskParams struct {
	ProviderRequestID string
	UserID            int64
	APIKeyID          int64
	Status            string
	Model             string
	Resolution        string
	DurationSeconds   int
}

type CreativeVideoTaskFilter struct {
	Status         string
	ExcludeDeleted bool
	Limit          int
	Offset         int
}

type CreativeVideoRepository interface {
	CreateCreativeVideoTask(ctx context.Context, params CreateCreativeVideoTaskParams) (*CreativeVideoTask, error)
	CompleteCreativeVideoTaskSubmit(ctx context.Context, params CompleteCreativeVideoTaskSubmitParams) error
	MarkCreativeVideoTaskFailed(ctx context.Context, taskID, code, message string) error
	ObserveCreativeVideoTask(ctx context.Context, params ObserveCreativeVideoTaskParams) error
	GetCreativeVideoTaskForOwner(ctx context.Context, userID, apiKeyID int64, requestID string) (*CreativeVideoTask, error)
	ListCreativeVideoTasksForOwner(ctx context.Context, userID, apiKeyID int64, filter CreativeVideoTaskFilter) ([]*CreativeVideoTask, error)
	CountActiveCreativeVideoTasksForUser(ctx context.Context, userID int64) (int, error)
	MarkCreativeVideoTaskDownloaded(ctx context.Context, userID, apiKeyID int64, providerRequestID string, downloadedAt time.Time) error
	MarkCreativeVideoTaskUserDeleted(ctx context.Context, userID, apiKeyID int64, providerRequestID string, deletedAt time.Time) error
	ListCreativeVideoTasksDueForRecordCleanup(ctx context.Context, cutoff time.Time, maxRecordsPerUser, limit int) ([]*CreativeVideoTask, error)
	MarkCreativeVideoTaskAutoDeleted(ctx context.Context, taskID string, deletedAt time.Time) error
}

type CreativeVideoTaskPublic struct {
	ID              string  `json:"id"`
	Object          string  `json:"object"`
	Status          string  `json:"status"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	PromptPreview   *string `json:"prompt_preview,omitempty"`
	Resolution      *string `json:"resolution,omitempty"`
	DurationSeconds *int    `json:"duration_seconds,omitempty"`
	CreatedAt       int64   `json:"created_at"`
	SubmittedAt     *int64  `json:"submitted_at,omitempty"`
	CompletedAt     *int64  `json:"completed_at,omitempty"`
	DownloadedAt    *int64  `json:"downloaded_at,omitempty"`
	OutputDeletedAt *int64  `json:"output_deleted_at,omitempty"`
}

type CreativeVideoTasksResponse struct {
	Object            string                    `json:"object"`
	Data              []CreativeVideoTaskPublic `json:"data"`
	HasMore           bool                      `json:"has_more"`
	RetentionDays     int                       `json:"retention_days"`
	MaxRecordsPerUser int                       `json:"max_records_per_user"`
	MaxRunningPerUser int                       `json:"max_running_per_user"`
}

type CreativeVideoTasksQuery struct {
	Status string
	Limit  int
	Cursor string
}

type CreativeVideoService struct {
	Repo              CreativeVideoRepository
	WorkbenchSettings CreativeWorkbenchSettingsReader
}

func NewCreativeVideoService(repo CreativeVideoRepository, settingReader CreativeWorkbenchSettingsReader) *CreativeVideoService {
	return &CreativeVideoService{Repo: repo, WorkbenchSettings: settingReader}
}

func (s *CreativeVideoService) CheckCreateAllowed(ctx context.Context, userID int64) error {
	settings := s.creativeWorkbenchSettings(ctx)
	if !settings.Enabled || !settings.VideoEnabled {
		return ErrCreativeVideoDisabled
	}
	if settings.VideoMaxRunningPerUser > 0 && s.Repo != nil {
		count, err := s.Repo.CountActiveCreativeVideoTasksForUser(ctx, userID)
		if err != nil {
			return err
		}
		if count >= settings.VideoMaxRunningPerUser {
			return ErrCreativeVideoRunningLimitExceeded
		}
	}
	return nil
}

func (s *CreativeVideoService) CreatePending(ctx context.Context, owner BatchImageOwner, body []byte, info GrokMediaRequestInfo) (*CreativeVideoTask, error) {
	if s == nil || s.Repo == nil {
		return nil, nil
	}
	settings := s.creativeWorkbenchSettings(ctx)
	if !settings.Enabled || !settings.VideoEnabled {
		return nil, ErrCreativeVideoDisabled
	}
	taskID, err := NewCreativeVideoTaskID()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().AddDate(0, 0, settings.RetentionDays)
	prompt := creativeVideoPromptPreview(info.Prompt)
	resolution := strings.TrimSpace(info.Resolution)
	duration := info.DurationSeconds
	return s.Repo.CreateCreativeVideoTask(ctx, CreateCreativeVideoTaskParams{
		TaskID:                taskID,
		UserID:                owner.UserID,
		APIKeyID:              owner.APIKeyID,
		GroupID:               owner.GroupID,
		Provider:              CreativeVideoProviderGrok,
		Model:                 strings.TrimSpace(info.Model),
		PromptPreview:         prompt,
		Status:                CreativeVideoStatusQueued,
		Resolution:            optionalStringPtr(resolution),
		DurationSeconds:       optionalIntPtr(duration),
		OutputExpiresAt:       &expiresAt,
		MaxActiveTasksPerUser: settings.VideoMaxRunningPerUser,
	})
}

func (s *CreativeVideoService) CreateProviderPending(ctx context.Context, owner BatchImageOwner, provider string, req CreativeVideoProviderRequest) (*CreativeVideoTask, error) {
	if s == nil || s.Repo == nil {
		return nil, nil
	}
	settings := s.creativeWorkbenchSettings(ctx)
	if !settings.Enabled || !settings.VideoEnabled {
		return nil, ErrCreativeVideoDisabled
	}
	taskID, err := NewCreativeVideoTaskID()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().AddDate(0, 0, settings.RetentionDays)
	return s.Repo.CreateCreativeVideoTask(ctx, CreateCreativeVideoTaskParams{
		TaskID:                taskID,
		UserID:                owner.UserID,
		APIKeyID:              owner.APIKeyID,
		GroupID:               owner.GroupID,
		Provider:              strings.TrimSpace(provider),
		Model:                 strings.TrimSpace(req.Model),
		PromptPreview:         creativeVideoPromptPreview(req.Prompt),
		Status:                CreativeVideoStatusQueued,
		Resolution:            optionalStringPtr(req.Resolution),
		DurationSeconds:       optionalIntPtr(req.Duration),
		OutputExpiresAt:       &expiresAt,
		MaxActiveTasksPerUser: settings.VideoMaxRunningPerUser,
	})
}

func (s *CreativeVideoService) CompleteSubmit(ctx context.Context, taskID string, accountID int64, result *OpenAIForwardResult, fallback GrokMediaRequestInfo) error {
	if s == nil || s.Repo == nil || strings.TrimSpace(taskID) == "" || result == nil {
		return nil
	}
	providerRequestID := strings.TrimSpace(result.ResponseID)
	if providerRequestID == "" {
		return infraerrors.New(http.StatusBadGateway, "CREATIVE_VIDEO_MISSING_REQUEST_ID", "creative video upstream response did not include a request id")
	}
	return s.Repo.CompleteCreativeVideoTaskSubmit(ctx, CompleteCreativeVideoTaskSubmitParams{
		TaskID:            taskID,
		ProviderRequestID: providerRequestID,
		AccountID:         accountID,
		Status:            CreativeVideoStatusRunning,
		Model:             firstNonEmpty(strings.TrimSpace(result.Model), strings.TrimSpace(fallback.Model)),
		Resolution:        firstNonEmpty(strings.TrimSpace(result.VideoResolution), strings.TrimSpace(fallback.Resolution)),
		DurationSeconds:   firstPositive(result.VideoDurationSeconds, fallback.DurationSeconds),
	})
}

func (s *CreativeVideoService) CompleteProviderSubmit(ctx context.Context, taskID string, accountID int64, status *CreativeVideoProviderStatus, fallback CreativeVideoProviderRequest) error {
	if s == nil || s.Repo == nil || strings.TrimSpace(taskID) == "" || status == nil {
		return nil
	}
	providerRequestID := strings.TrimSpace(status.ID)
	if providerRequestID == "" {
		return infraerrors.New(http.StatusBadGateway, "CREATIVE_VIDEO_MISSING_REQUEST_ID", "creative video upstream response did not include a request id")
	}
	nextStatus := CreativeVideoStatusRunning
	if status.Status == CreativeVideoStatusCompleted {
		nextStatus = CreativeVideoStatusCompleted
	}
	return s.Repo.CompleteCreativeVideoTaskSubmit(ctx, CompleteCreativeVideoTaskSubmitParams{
		TaskID:            taskID,
		ProviderRequestID: providerRequestID,
		AccountID:         accountID,
		Status:            nextStatus,
		Model:             firstNonEmpty(strings.TrimSpace(status.Model), strings.TrimSpace(fallback.Model)),
		Resolution:        firstNonEmpty(strings.TrimSpace(status.Resolution), strings.TrimSpace(fallback.Resolution)),
		DurationSeconds:   firstPositive(status.DurationSeconds, fallback.Duration),
	})
}

func (s *CreativeVideoService) FailTask(ctx context.Context, taskID, code, message string) {
	if s == nil || s.Repo == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	_ = s.Repo.MarkCreativeVideoTaskFailed(ctx, taskID, code, message)
}

func (s *CreativeVideoService) ObserveProviderStatus(ctx context.Context, owner BatchImageOwner, requestID string, status *CreativeVideoProviderStatus) {
	if s == nil || s.Repo == nil || status == nil || strings.TrimSpace(requestID) == "" {
		return
	}
	nextStatus := strings.TrimSpace(status.Status)
	if nextStatus == "" {
		return
	}
	_ = s.Repo.ObserveCreativeVideoTask(ctx, ObserveCreativeVideoTaskParams{
		ProviderRequestID: strings.TrimSpace(requestID),
		UserID:            owner.UserID,
		APIKeyID:          owner.APIKeyID,
		Status:            nextStatus,
		Model:             strings.TrimSpace(status.Model),
		Resolution:        strings.TrimSpace(status.Resolution),
		DurationSeconds:   status.DurationSeconds,
	})
}

func (s *CreativeVideoService) GetTask(ctx context.Context, owner BatchImageOwner, requestID string) (*CreativeVideoTask, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrCreativeVideoTaskNotFound
	}
	return s.Repo.GetCreativeVideoTaskForOwner(ctx, owner.UserID, owner.APIKeyID, requestID)
}

func (s *CreativeVideoService) ObserveGatewayResult(ctx context.Context, userID, apiKeyID int64, providerRequestID string, result *OpenAIForwardResult) {
	if s == nil || s.Repo == nil || result == nil || strings.TrimSpace(providerRequestID) == "" {
		return
	}
	status := creativeVideoStatusFromGrokStatus(result.VideoStatus, result.VideoCount > 0)
	if status == "" {
		return
	}
	_ = s.Repo.ObserveCreativeVideoTask(ctx, ObserveCreativeVideoTaskParams{
		ProviderRequestID: strings.TrimSpace(providerRequestID),
		UserID:            userID,
		APIKeyID:          apiKeyID,
		Status:            status,
		Model:             strings.TrimSpace(result.Model),
		Resolution:        strings.TrimSpace(result.VideoResolution),
		DurationSeconds:   result.VideoDurationSeconds,
	})
}

func (s *CreativeVideoService) List(ctx context.Context, owner BatchImageOwner, query CreativeVideoTasksQuery) (*CreativeVideoTasksResponse, error) {
	settings := s.creativeWorkbenchSettings(ctx)
	if s == nil || s.Repo == nil {
		return creativeVideoTasksResponseWithSettings(settings, nil, false), nil
	}
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	offset := 0
	if strings.TrimSpace(query.Cursor) != "" {
		if parsed, err := strconvAtoi(query.Cursor); err == nil && parsed > 0 {
			offset = parsed
		}
	}
	tasks, err := s.Repo.ListCreativeVideoTasksForOwner(ctx, owner.UserID, owner.APIKeyID, CreativeVideoTaskFilter{
		Status:         strings.TrimSpace(query.Status),
		ExcludeDeleted: true,
		Limit:          limit + 1,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(tasks) > limit
	if hasMore {
		tasks = tasks[:limit]
	}
	data := make([]CreativeVideoTaskPublic, 0, len(tasks))
	for _, task := range tasks {
		data = append(data, CreativeVideoTaskToPublic(task))
	}
	return creativeVideoTasksResponseWithSettings(settings, data, hasMore), nil
}

func creativeVideoTasksResponseWithSettings(settings *CreativeWorkbenchSettings, data []CreativeVideoTaskPublic, hasMore bool) *CreativeVideoTasksResponse {
	if settings == nil {
		settings = DefaultCreativeWorkbenchSettings()
	}
	if data == nil {
		data = []CreativeVideoTaskPublic{}
	}
	return &CreativeVideoTasksResponse{
		Object:            "list",
		Data:              data,
		HasMore:           hasMore,
		RetentionDays:     settings.RetentionDays,
		MaxRecordsPerUser: settings.MaxRecordsPerUser,
		MaxRunningPerUser: settings.VideoMaxRunningPerUser,
	}
}

func (s *CreativeVideoService) MarkDownloaded(ctx context.Context, owner BatchImageOwner, requestID string) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	return s.Repo.MarkCreativeVideoTaskDownloaded(ctx, owner.UserID, owner.APIKeyID, requestID, time.Now())
}

func (s *CreativeVideoService) DeleteRecord(ctx context.Context, owner BatchImageOwner, requestID string) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	return s.Repo.MarkCreativeVideoTaskUserDeleted(ctx, owner.UserID, owner.APIKeyID, requestID, time.Now())
}

func (s *CreativeVideoService) CleanupOnce(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.Repo == nil {
		return 0, nil
	}
	settings := s.creativeWorkbenchSettings(ctx)
	if !settings.AutoCleanupEnabled {
		return 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	cutoff := now.AddDate(0, 0, -settings.RetentionDays)
	tasks, err := s.Repo.ListCreativeVideoTasksDueForRecordCleanup(ctx, cutoff, settings.MaxRecordsPerUser, limit)
	if err != nil {
		return 0, err
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if err := s.Repo.MarkCreativeVideoTaskAutoDeleted(ctx, task.TaskID, now); err != nil {
			return 0, err
		}
	}
	return len(tasks), nil
}

func CreativeVideoTaskToPublic(task *CreativeVideoTask) CreativeVideoTaskPublic {
	if task == nil {
		return CreativeVideoTaskPublic{}
	}
	return CreativeVideoTaskPublic{
		ID:              firstNonEmpty(ptrString(task.ProviderRequestID), task.TaskID),
		Object:          "creative.video.task",
		Status:          task.Status,
		Provider:        task.Provider,
		Model:           task.Model,
		PromptPreview:   task.PromptPreview,
		Resolution:      task.Resolution,
		DurationSeconds: task.DurationSeconds,
		CreatedAt:       task.CreatedAt.Unix(),
		SubmittedAt:     unixPtr(task.SubmittedAt),
		CompletedAt:     unixPtr(task.CompletedAt),
		DownloadedAt:    unixPtr(task.DownloadedAt),
		OutputDeletedAt: unixPtr(task.OutputDeletedAt),
	}
}

func NewCreativeVideoTaskID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "vidtask_" + hex.EncodeToString(b[:]), nil
}

func IsCreativeWorkbenchVideoHeader(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), CreativeWorkbenchHeaderVideo)
}

func ExtractGrokVideoStatus(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "status").String())
}

func creativeVideoStatusFromGrokStatus(status string, billableDone bool) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done":
		if billableDone {
			return CreativeVideoStatusCompleted
		}
		return ""
	case "pending", "running", "queued", "processing":
		return CreativeVideoStatusRunning
	case "failed", "error":
		return CreativeVideoStatusFailed
	case "expired":
		return CreativeVideoStatusExpired
	default:
		if billableDone {
			return CreativeVideoStatusCompleted
		}
		return ""
	}
}

func (s *CreativeVideoService) creativeWorkbenchSettings(ctx context.Context) *CreativeWorkbenchSettings {
	if s == nil || s.WorkbenchSettings == nil {
		return DefaultCreativeWorkbenchSettings()
	}
	settings, err := s.WorkbenchSettings.GetCreativeWorkbenchSettings(ctx)
	if err != nil || settings == nil {
		return DefaultCreativeWorkbenchSettings()
	}
	normalizeCreativeWorkbenchSettings(settings)
	return settings
}

func creativeVideoPromptPreview(prompt string) *string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}
	const maxRunes = 240
	runes := []rune(prompt)
	if len(runes) > maxRunes {
		prompt = string(runes[:maxRunes])
	}
	return &prompt
}

func optionalStringPtr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func optionalIntPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func ptrString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func unixPtr(t *time.Time) *int64 {
	if t == nil || t.IsZero() {
		return nil
	}
	seconds := t.Unix()
	return &seconds
}

func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func strconvAtoi(value string) (int, error) {
	value = strings.TrimSpace(value)
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, infraerrors.New(http.StatusBadRequest, "INVALID_CURSOR", "invalid cursor")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
