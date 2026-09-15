package service

import (
	"context"
	"testing"
	"time"

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

type fakeCreativeWorkbenchReader struct {
	settings *CreativeWorkbenchSettings
}

func (r fakeCreativeWorkbenchReader) GetCreativeWorkbenchSettings(context.Context) (*CreativeWorkbenchSettings, error) {
	return r.settings, nil
}

type fakeCreativeVideoRepo struct {
	activeCount        int
	observedStatus     string
	dueTasks           []*CreativeVideoTask
	cleanupCutoff      time.Time
	autoDeletedTaskIDs []string
}

func (r *fakeCreativeVideoRepo) CreateCreativeVideoTask(_ context.Context, params CreateCreativeVideoTaskParams) (*CreativeVideoTask, error) {
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

func (r *fakeCreativeVideoRepo) ListCreativeVideoTasksForOwner(context.Context, int64, int64, CreativeVideoTaskFilter) ([]*CreativeVideoTask, error) {
	return nil, nil
}

func (r *fakeCreativeVideoRepo) CountActiveCreativeVideoTasksForUser(context.Context, int64) (int, error) {
	return r.activeCount, nil
}

func (r *fakeCreativeVideoRepo) MarkCreativeVideoTaskDownloaded(context.Context, int64, int64, string, time.Time) error {
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
