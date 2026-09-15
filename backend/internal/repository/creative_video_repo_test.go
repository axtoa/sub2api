package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCreativeVideoRecordCleanupOnlySelectsTerminalTasks(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &creativeVideoRepository{sql: db}
	cutoff := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH ranked AS.*t\.status IN \('completed', 'failed', 'expired', 'output_deleted'\).*t\.created_at <= \$1`).
		WithArgs(cutoff, 50, 25).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.ListCreativeVideoTasksDueForRecordCleanup(context.Background(), cutoff, 50, 25)
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreativeVideoAutoDeleteRequiresTerminalStatus(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &creativeVideoRepository{sql: db}
	mock.ExpectExec(`(?s)UPDATE creative_video_tasks.*status IN \('completed', 'failed', 'expired', 'output_deleted'\)`).
		WithArgs("vidtask_1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.MarkCreativeVideoTaskAutoDeleted(context.Background(), "vidtask_1", time.Now())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
