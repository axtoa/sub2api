package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creativeVideoSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type creativeVideoRepository struct {
	db  *sql.DB
	sql creativeVideoSQLExecutor
}

func NewCreativeVideoRepository(db *sql.DB) service.CreativeVideoRepository {
	return &creativeVideoRepository{db: db, sql: db}
}

func (r *creativeVideoRepository) CreateCreativeVideoTask(ctx context.Context, params service.CreateCreativeVideoTaskParams) (*service.CreativeVideoTask, error) {
	if strings.TrimSpace(params.Provider) == "" {
		params.Provider = service.CreativeVideoProviderGrok
	}
	if strings.TrimSpace(params.Status) == "" {
		params.Status = service.CreativeVideoStatusQueued
	}
	if strings.TrimSpace(params.TaskID) == "" {
		taskID, err := service.NewCreativeVideoTaskID()
		if err != nil {
			return nil, err
		}
		params.TaskID = taskID
	}
	var task *service.CreativeVideoTask
	var err error
	if params.MaxActiveTasksPerUser > 0 {
		task, err = r.createCreativeVideoTaskWithActiveLimit(ctx, params)
	} else {
		task, err = createCreativeVideoTaskWithSQL(ctx, r.sql, params)
	}
	if err != nil {
		if errors.Is(err, service.ErrCreativeVideoRunningLimitExceeded) {
			return nil, err
		}
		return nil, translatePersistenceError(err, nil, nil)
	}
	return task, nil
}

func (r *creativeVideoRepository) createCreativeVideoTaskWithActiveLimit(ctx context.Context, params service.CreateCreativeVideoTaskParams) (*service.CreativeVideoTask, error) {
	if r.db == nil {
		return createCreativeVideoTaskWithActiveLimitSQL(ctx, r.sql, params)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	task, err := createCreativeVideoTaskWithActiveLimitSQL(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return task, nil
}

func createCreativeVideoTaskWithActiveLimitSQL(ctx context.Context, sqlq creativeVideoSQLExecutor, params service.CreateCreativeVideoTaskParams) (*service.CreativeVideoTask, error) {
	if params.UserID <= 0 || params.MaxActiveTasksPerUser <= 0 {
		return createCreativeVideoTaskWithSQL(ctx, sqlq, params)
	}
	rows, err := sqlq.QueryContext(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockHash("creative_workbench:video:"+strconv.FormatInt(params.UserID, 10)))
	if err != nil {
		return nil, err
	}
	_ = rows.Close()

	var activeCount int
	if err := sqlq.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM creative_video_tasks
WHERE user_id = $1
  AND user_deleted_at IS NULL
  AND status IN ('queued', 'submitted', 'running')`, params.UserID).Scan(&activeCount); err != nil {
		return nil, err
	}
	if activeCount >= params.MaxActiveTasksPerUser {
		return nil, service.ErrCreativeVideoRunningLimitExceeded
	}
	return createCreativeVideoTaskWithSQL(ctx, sqlq, params)
}

func createCreativeVideoTaskWithSQL(ctx context.Context, sqlq creativeVideoSQLExecutor, params service.CreateCreativeVideoTaskParams) (*service.CreativeVideoTask, error) {
	return scanCreativeVideoTask(sqlq.QueryRowContext(ctx, `
INSERT INTO creative_video_tasks (
    task_id, user_id, api_key_id, group_id, provider, model, prompt_preview,
    status, resolution, duration_seconds, output_expires_at, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, NOW(), NOW()
)
RETURNING `+creativeVideoTaskSelectColumns,
		params.TaskID,
		params.UserID,
		params.APIKeyID,
		params.GroupID,
		params.Provider,
		params.Model,
		params.PromptPreview,
		params.Status,
		params.Resolution,
		params.DurationSeconds,
		params.OutputExpiresAt,
	))
}

func (r *creativeVideoRepository) CompleteCreativeVideoTaskSubmit(ctx context.Context, params service.CompleteCreativeVideoTaskSubmitParams) error {
	if strings.TrimSpace(params.TaskID) == "" || strings.TrimSpace(params.ProviderRequestID) == "" {
		return nil
	}
	now := time.Now()
	_, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET provider_request_id = $2,
    account_id = NULLIF($3, 0),
    status = $4,
    model = COALESCE(NULLIF($5, ''), model),
    resolution = COALESCE(NULLIF($6, ''), resolution),
    duration_seconds = COALESCE(NULLIF($7, 0), duration_seconds),
    download_url = COALESCE(NULLIF($8, ''), download_url),
    file_id = COALESCE(NULLIF($9, ''), file_id),
    completed_at = CASE WHEN $4 IN ('completed', 'failed', 'expired', 'output_deleted') THEN COALESCE(completed_at, $10) ELSE completed_at END,
    submitted_at = COALESCE(submitted_at, $10),
    updated_at = $10
WHERE task_id = $1
  AND user_deleted_at IS NULL`, params.TaskID, params.ProviderRequestID, params.AccountID, params.Status, params.Model, params.Resolution, params.DurationSeconds, params.DownloadURL, params.FileID, now)
	return translatePersistenceError(err, nil, nil)
}

func (r *creativeVideoRepository) MarkCreativeVideoTaskFailed(ctx context.Context, taskID, code, message string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	now := time.Now()
	_, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET status = 'failed',
    last_error_code = NULLIF($2, ''),
    last_error_message = NULLIF($3, ''),
    completed_at = COALESCE(completed_at, $4),
    updated_at = $4
WHERE task_id = $1
  AND user_deleted_at IS NULL
  AND status IN ('queued', 'submitted', 'running')`, taskID, code, message, now)
	return translatePersistenceError(err, nil, nil)
}

func (r *creativeVideoRepository) ObserveCreativeVideoTask(ctx context.Context, params service.ObserveCreativeVideoTaskParams) error {
	requestID := strings.TrimSpace(params.ProviderRequestID)
	if requestID == "" {
		return nil
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		return nil
	}
	now := time.Now()
	_, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET status = $4,
    model = COALESCE(NULLIF($5, ''), model),
    resolution = COALESCE(NULLIF($6, ''), resolution),
    duration_seconds = COALESCE(NULLIF($7, 0), duration_seconds),
    download_url = COALESCE(NULLIF($9, ''), download_url),
    file_id = COALESCE(NULLIF($10, ''), file_id),
    completed_at = CASE WHEN $4 IN ('completed', 'failed', 'expired', 'output_deleted') THEN COALESCE(completed_at, $11) ELSE completed_at END,
    updated_at = $11
WHERE (provider_request_id = $1 OR (NULLIF($8, '') IS NOT NULL AND task_id = $8))
  AND user_id = $2
  AND api_key_id = $3
  AND user_deleted_at IS NULL`, requestID, params.UserID, params.APIKeyID, status, params.Model, params.Resolution, params.DurationSeconds, strings.TrimSpace(params.TaskID), params.DownloadURL, params.FileID, now)
	return translatePersistenceError(err, nil, nil)
}

func (r *creativeVideoRepository) GetCreativeVideoTaskForOwner(ctx context.Context, userID, apiKeyID int64, requestID string) (*service.CreativeVideoTask, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, service.ErrCreativeVideoTaskNotFound
	}
	task, err := scanCreativeVideoTask(r.sql.QueryRowContext(ctx, `
SELECT `+creativeVideoTaskSelectColumns+`
FROM creative_video_tasks
WHERE (provider_request_id = $1 OR task_id = $1)
  AND user_id = $2
  AND api_key_id = $3
  AND user_deleted_at IS NULL`, requestID, userID, apiKeyID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreativeVideoTaskNotFound
	}
	if err != nil {
		return nil, translatePersistenceError(err, nil, nil)
	}
	return task, nil
}

func (r *creativeVideoRepository) ListCreativeVideoTasksForOwner(ctx context.Context, userID, apiKeyID int64, filter service.CreativeVideoTaskFilter) ([]*service.CreativeVideoTask, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 501 {
		limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	query := `SELECT ` + creativeVideoTaskSelectColumns + `
FROM creative_video_tasks
WHERE user_id = $1 AND api_key_id = $2`
	args := []any{userID, apiKeyID}
	if filter.ExcludeDeleted {
		query += " AND user_deleted_at IS NULL"
	}
	if strings.TrimSpace(filter.Status) != "" {
		query += " AND status = $" + strconv.Itoa(len(args)+1)
		args = append(args, strings.TrimSpace(filter.Status))
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, filter.Offset)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCreativeVideoTasks(rows)
}

func (r *creativeVideoRepository) CountActiveCreativeVideoTasksForUser(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.sql.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM creative_video_tasks
WHERE user_id = $1
  AND user_deleted_at IS NULL
  AND status IN ('queued', 'submitted', 'running')`, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *creativeVideoRepository) MarkCreativeVideoTaskDownloaded(ctx context.Context, userID, apiKeyID int64, providerRequestID string, downloadedAt time.Time) error {
	res, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET downloaded_at = $4,
    updated_at = $4
WHERE (provider_request_id = $1 OR task_id = $1)
  AND user_id = $2
  AND api_key_id = $3
  AND user_deleted_at IS NULL`, strings.TrimSpace(providerRequestID), userID, apiKeyID, downloadedAt)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	return requireRowsAffected(res, service.ErrCreativeVideoTaskNotFound)
}

func (r *creativeVideoRepository) MarkCreativeVideoTaskUsage(ctx context.Context, userID, apiKeyID int64, providerRequestID string, actualCost float64) error {
	res, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET actual_cost = $4,
    updated_at = NOW()
WHERE (provider_request_id = $1 OR task_id = $1)
  AND user_id = $2
  AND api_key_id = $3
  AND user_deleted_at IS NULL`, strings.TrimSpace(providerRequestID), userID, apiKeyID, actualCost)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	return requireRowsAffected(res, service.ErrCreativeVideoTaskNotFound)
}

func (r *creativeVideoRepository) MarkCreativeVideoTaskOutputMetadata(ctx context.Context, userID, apiKeyID int64, providerRequestID string, fileSizeBytes int64, contentType string) error {
	if fileSizeBytes < 0 {
		fileSizeBytes = 0
	}
	res, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET file_size_bytes = NULLIF($4, 0),
    content_type = NULLIF($5, ''),
    updated_at = NOW()
WHERE (provider_request_id = $1 OR task_id = $1)
  AND user_id = $2
  AND api_key_id = $3
  AND user_deleted_at IS NULL`, strings.TrimSpace(providerRequestID), userID, apiKeyID, fileSizeBytes, strings.TrimSpace(contentType))
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	return requireRowsAffected(res, service.ErrCreativeVideoTaskNotFound)
}

func (r *creativeVideoRepository) MarkCreativeVideoTaskUserDeleted(ctx context.Context, userID, apiKeyID int64, providerRequestID string, deletedAt time.Time) error {
	res, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET user_deleted_at = $4,
    output_deleted_at = COALESCE(output_deleted_at, $4),
    status = CASE WHEN status = 'completed' THEN 'output_deleted' ELSE status END,
    updated_at = $4
WHERE (provider_request_id = $1 OR task_id = $1)
  AND user_id = $2
  AND api_key_id = $3
  AND user_deleted_at IS NULL`, strings.TrimSpace(providerRequestID), userID, apiKeyID, deletedAt)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	return requireRowsAffected(res, service.ErrCreativeVideoTaskNotFound)
}

func (r *creativeVideoRepository) ListCreativeVideoTasksDueForRecordCleanup(ctx context.Context, cutoff time.Time, maxRecordsPerUser, limit int) ([]*service.CreativeVideoTask, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if maxRecordsPerUser <= 0 {
		maxRecordsPerUser = service.CreativeWorkbenchMaxRecordsDefault
	}
	rows, err := r.sql.QueryContext(ctx, `
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS rn
    FROM creative_video_tasks
    WHERE user_deleted_at IS NULL
),
due AS (
    SELECT t.id
    FROM creative_video_tasks t
    LEFT JOIN ranked r ON r.id = t.id
    WHERE t.user_deleted_at IS NULL
      AND t.status IN ('completed', 'failed', 'expired', 'output_deleted')
      AND (
          t.created_at <= $1
          OR COALESCE(r.rn, 0) > $2
      )
    ORDER BY t.created_at ASC, t.id ASC
    LIMIT $3
)
SELECT `+creativeVideoTaskSelectColumns+`
FROM creative_video_tasks
WHERE id IN (SELECT id FROM due)
ORDER BY created_at ASC, id ASC`, cutoff, maxRecordsPerUser, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCreativeVideoTasks(rows)
}

func (r *creativeVideoRepository) MarkCreativeVideoTaskAutoDeleted(ctx context.Context, taskID string, deletedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE creative_video_tasks
SET user_deleted_at = $2,
    output_deleted_at = COALESCE(output_deleted_at, $2),
    status = CASE WHEN status = 'completed' THEN 'output_deleted' ELSE status END,
    updated_at = $2
WHERE task_id = $1
  AND user_deleted_at IS NULL
  AND status IN ('completed', 'failed', 'expired', 'output_deleted')`, strings.TrimSpace(taskID), deletedAt)
	return translatePersistenceError(err, nil, nil)
}

const creativeVideoTaskSelectColumns = `
id, task_id, provider_request_id, user_id, api_key_id, group_id, account_id, provider, model,
prompt_preview, status, resolution, duration_seconds, output_expires_at, downloaded_at,
output_deleted_at, user_deleted_at, last_error_code, last_error_message, created_at, updated_at,
submitted_at, completed_at, actual_cost, file_size_bytes, content_type, download_url, file_id`

func scanCreativeVideoTasks(rows *sql.Rows) ([]*service.CreativeVideoTask, error) {
	tasks := []*service.CreativeVideoTask{}
	for rows.Next() {
		task, err := scanCreativeVideoTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func scanCreativeVideoTask(row rowScanner) (*service.CreativeVideoTask, error) {
	var task service.CreativeVideoTask
	var providerRequestID, promptPreview, resolution, lastErrorCode, lastErrorMessage, contentType, downloadURL, fileID sql.NullString
	var groupID, accountID sql.NullInt64
	var durationSeconds, fileSizeBytes sql.NullInt64
	var actualCost sql.NullFloat64
	var outputExpiresAt, downloadedAt, outputDeletedAt, userDeletedAt, submittedAt, completedAt sql.NullTime
	if err := row.Scan(
		&task.ID,
		&task.TaskID,
		&providerRequestID,
		&task.UserID,
		&task.APIKeyID,
		&groupID,
		&accountID,
		&task.Provider,
		&task.Model,
		&promptPreview,
		&task.Status,
		&resolution,
		&durationSeconds,
		&outputExpiresAt,
		&downloadedAt,
		&outputDeletedAt,
		&userDeletedAt,
		&lastErrorCode,
		&lastErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&submittedAt,
		&completedAt,
		&actualCost,
		&fileSizeBytes,
		&contentType,
		&downloadURL,
		&fileID,
	); err != nil {
		return nil, err
	}
	task.ProviderRequestID = nullStringPtr(providerRequestID)
	task.GroupID = creativeNullInt64Ptr(groupID)
	task.AccountID = creativeNullInt64Ptr(accountID)
	task.PromptPreview = nullStringPtr(promptPreview)
	task.Resolution = nullStringPtr(resolution)
	if durationSeconds.Valid {
		v := int(durationSeconds.Int64)
		task.DurationSeconds = &v
	}
	if actualCost.Valid {
		v := actualCost.Float64
		task.ActualCost = &v
	}
	if fileSizeBytes.Valid {
		v := fileSizeBytes.Int64
		task.FileSizeBytes = &v
	}
	task.ContentType = nullStringPtr(contentType)
	task.DownloadURL = nullStringPtr(downloadURL)
	task.FileID = nullStringPtr(fileID)
	task.OutputExpiresAt = nullTimePtr(outputExpiresAt)
	task.DownloadedAt = nullTimePtr(downloadedAt)
	task.OutputDeletedAt = nullTimePtr(outputDeletedAt)
	task.UserDeletedAt = nullTimePtr(userDeletedAt)
	task.LastErrorCode = nullStringPtr(lastErrorCode)
	task.LastErrorMessage = nullStringPtr(lastErrorMessage)
	task.SubmittedAt = nullTimePtr(submittedAt)
	task.CompletedAt = nullTimePtr(completedAt)
	return &task, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func creativeNullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

func requireRowsAffected(res sql.Result, notFound error) error {
	if res == nil {
		return nil
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFound
	}
	return nil
}
