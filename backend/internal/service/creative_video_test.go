package service

import (
	"context"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCreativeVideoServiceCheckCreateAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := &fakeCreativeVideoRepo{activeCount: 4}
	svc := NewCreativeVideoService(repo, fakeCreativeWorkbenchReader{settings: &CreativeWorkbenchSettings{
		Enabled:                true,
		VideoEnabled:           true,
		AutoCleanupEnabled:     true,
		RetentionDays:          3,
		MaxRecordsPerUser:      50,
		ImageMaxRunningPerUser: 10,
		VideoMaxRunningPerUser: 5,
	}})
	require.NoError(t, svc.CheckCreateAllowed(ctx, 10))

	repo.activeCount = 5
	require.ErrorIs(t, svc.CheckCreateAllowed(ctx, 10), ErrCreativeVideoRunningLimitExceeded)

	svc.WorkbenchSettings = fakeCreativeWorkbenchReader{settings: &CreativeWorkbenchSettings{
		Enabled:                true,
		VideoEnabled:           false,
		AutoCleanupEnabled:     true,
		RetentionDays:          3,
		MaxRecordsPerUser:      50,
		ImageMaxRunningPerUser: 10,
		VideoMaxRunningPerUser: 5,
	}}
	require.ErrorIs(t, svc.CheckCreateAllowed(ctx, 10), ErrCreativeVideoDisabled)
}

func TestCreativeVideoObserveRequiresDoneWithVideoURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeCreativeVideoRepo{}
	svc := NewCreativeVideoService(repo, fakeCreativeWorkbenchReader{settings: DefaultCreativeWorkbenchSettings()})

	svc.ObserveGatewayResult(ctx, 1, 2, "req-1", &OpenAIForwardResult{VideoStatus: "done"})
	require.Empty(t, repo.observedStatus)

	svc.ObserveGatewayResult(ctx, 1, 2, "req-1", &OpenAIForwardResult{VideoStatus: "done", VideoCount: 1})
	require.Equal(t, CreativeVideoStatusCompleted, repo.observedStatus)
}

func TestCreativeVideoCompleteSubmitRejectsMissingRequestID(t *testing.T) {
	t.Parallel()
	svc := NewCreativeVideoService(&fakeCreativeVideoRepo{}, fakeCreativeWorkbenchReader{settings: DefaultCreativeWorkbenchSettings()})

	err := svc.CompleteSubmit(context.Background(), "vidtask_1", 7, &OpenAIForwardResult{}, GrokMediaRequestInfo{
		Model: "grok-imagine-video",
	})
	require.Error(t, err)
	require.Equal(t, "CREATIVE_VIDEO_MISSING_REQUEST_ID", infraerrors.Reason(err))
}

func TestCreativeVideoCreateProviderPendingWrapsPersistenceFailure(t *testing.T) {
	t.Parallel()
	repoErr := errors.New("column file_id does not exist")
	repo := &fakeCreativeVideoRepo{createErr: repoErr}
	svc := NewCreativeVideoService(repo, fakeCreativeWorkbenchReader{settings: &CreativeWorkbenchSettings{
		Enabled:                true,
		VideoEnabled:           true,
		RetentionDays:          3,
		MaxRecordsPerUser:      50,
		VideoMaxRunningPerUser: 5,
	}})

	_, err := svc.CreateProviderPending(context.Background(), BatchImageOwner{
		UserID:   1,
		APIKeyID: 2,
	}, CreativeVideoProviderMiniMax, CreativeVideoProviderRequest{
		Model:  "MiniMax-H3",
		Prompt: "a rainy street",
	})

	require.ErrorIs(t, err, ErrCreativeVideoTaskPersistence)
	require.ErrorContains(t, err, repoErr.Error())
}

func TestCreativeVideoCleanupSoftDeletesRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	repo := &fakeCreativeVideoRepo{
		dueTasks: []*CreativeVideoTask{
			{TaskID: "vidtask_old"},
		},
	}
	svc := NewCreativeVideoService(repo, fakeCreativeWorkbenchReader{settings: &CreativeWorkbenchSettings{
		Enabled:                true,
		VideoEnabled:           true,
		AutoCleanupEnabled:     true,
		RetentionDays:          3,
		MaxRecordsPerUser:      50,
		ImageMaxRunningPerUser: 10,
		VideoMaxRunningPerUser: 5,
	}})

	deleted, err := svc.CleanupOnce(ctx, now, 100)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, []string{"vidtask_old"}, repo.autoDeletedTaskIDs)
	require.True(t, repo.cleanupCutoff.Equal(now.AddDate(0, 0, -3)))
}

func TestCreativeVideoListIncludesRuntimeLimits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeCreativeVideoRepo{
		listTasks: []*CreativeVideoTask{
			{
				TaskID:    "vidtask_1",
				UserID:    1,
				APIKeyID:  2,
				Provider:  CreativeVideoProviderGrok,
				Model:     "grok-imagine-video",
				Status:    CreativeVideoStatusRunning,
				CreatedAt: time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	svc := NewCreativeVideoService(repo, fakeCreativeWorkbenchReader{settings: &CreativeWorkbenchSettings{
		Enabled:                true,
		VideoEnabled:           true,
		AutoCleanupEnabled:     true,
		RetentionDays:          7,
		MaxRecordsPerUser:      80,
		ImageMaxRunningPerUser: 10,
		VideoMaxRunningPerUser: 6,
	}})

	got, err := svc.List(ctx, BatchImageOwner{UserID: 1, APIKeyID: 2}, CreativeVideoTasksQuery{Limit: 500})
	require.NoError(t, err)
	require.Len(t, got.Data, 1)
	require.Equal(t, 7, got.RetentionDays)
	require.Equal(t, 80, got.MaxRecordsPerUser)
	require.Equal(t, 6, got.MaxRunningPerUser)
	require.Equal(t, 501, repo.listFilter.Limit)
}

type fakeCreativeWorkbenchReader struct {
	settings *CreativeWorkbenchSettings
}

func (r fakeCreativeWorkbenchReader) GetCreativeWorkbenchSettings(context.Context) (*CreativeWorkbenchSettings, error) {
	return r.settings, nil
}

type fakeCreativeVideoRepo struct {
	activeCount        int
	createErr          error
	observedStatus     string
	listTasks          []*CreativeVideoTask
	listFilter         CreativeVideoTaskFilter
	dueTasks           []*CreativeVideoTask
	cleanupCutoff      time.Time
	autoDeletedTaskIDs []string
}

func (r *fakeCreativeVideoRepo) CreateCreativeVideoTask(_ context.Context, params CreateCreativeVideoTaskParams) (*CreativeVideoTask, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	if r.activeCount >= params.MaxActiveTasksPerUser && params.MaxActiveTasksPerUser > 0 {
		return nil, ErrCreativeVideoRunningLimitExceeded
	}
	return &CreativeVideoTask{TaskID: params.TaskID, UserID: params.UserID, APIKeyID: params.APIKeyID, Status: params.Status}, nil
}

func (r *fakeCreativeVideoRepo) CompleteCreativeVideoTaskSubmit(context.Context, CompleteCreativeVideoTaskSubmitParams) error {
	return nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskFailed(context.Context, string, string, string) error {
	return nil
}

func (r *fakeCreativeVideoRepo) ObserveCreativeVideoTask(_ context.Context, params ObserveCreativeVideoTaskParams) error {
	r.observedStatus = params.Status
	return nil
}

func (r *fakeCreativeVideoRepo) GetCreativeVideoTaskForOwner(_ context.Context, _ int64, _ int64, requestID string) (*CreativeVideoTask, error) {
	for _, task := range r.listTasks {
		if task == nil {
			continue
		}
		if task.TaskID == requestID || ptrString(task.ProviderRequestID) == requestID {
			return task, nil
		}
	}
	return nil, ErrCreativeVideoTaskNotFound
}

func (r *fakeCreativeVideoRepo) ListCreativeVideoTasksForOwner(_ context.Context, _ int64, _ int64, filter CreativeVideoTaskFilter) ([]*CreativeVideoTask, error) {
	r.listFilter = filter
	return r.listTasks, nil
}

func (r *fakeCreativeVideoRepo) CountActiveCreativeVideoTasksForUser(context.Context, int64) (int, error) {
	return r.activeCount, nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskDownloaded(context.Context, int64, int64, string, time.Time) error {
	return nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskUsage(context.Context, int64, int64, string, float64) error {
	return nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskOutputMetadata(context.Context, int64, int64, string, int64, string) error {
	return nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskUserDeleted(context.Context, int64, int64, string, time.Time) error {
	return nil
}

func (r *fakeCreativeVideoRepo) ListCreativeVideoTasksDueForRecordCleanup(_ context.Context, cutoff time.Time, _ int, _ int) ([]*CreativeVideoTask, error) {
	r.cleanupCutoff = cutoff
	return r.dueTasks, nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskAutoDeleted(_ context.Context, taskID string, _ time.Time) error {
	r.autoDeletedTaskIDs = append(r.autoDeletedTaskIDs, taskID)
	return nil
}
