//go:build unit

package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// The batch repository uses hand-written SQL: check both the appended INSERT
// argument and its SELECT/Scan position so restarts preserve the billing snapshot.
func TestBatchImageTokenMultiplierPersistence(t *testing.T) {
	for _, multiplier := range []float64{0, 2.5} {
		t.Run(fmt.Sprint(multiplier), func(t *testing.T) {
			want := multiplier
			if want == 0 {
				want = 1
			}
			var captured []string
			db, mock := newSQLCapturingMock(t, &captured)
			columns := strings.Split(batchImageJobColumns, ",")
			values := make([]driver.Value, len(columns))
			now := time.Now().UTC()
			for i := range columns {
				columns[i] = strings.TrimSpace(columns[i])
				switch columns[i] {
				case "id", "user_id", "item_count", "success_count", "fail_count", "cancelled_count", "retry_count", "version", "pricing_snapshot_version":
					values[i] = int64(1)
				case "estimated_cost", "base_unit_price", "group_rate_multiplier", "account_rate_multiplier", "batch_discount_multiplier", "hold_multiplier", "billable_unit_price", "hold_unit_price":
					values[i] = float64(1)
				case "batch_id", "provider", "model", "task_name", "status", "currency":
					values[i] = "test"
				case "created_at", "updated_at":
					values[i] = now
				case "token_usage_multiplier":
					values[i] = want
				}
			}
			args := make([]driver.Value, 38)
			for i := range args {
				args[i] = sqlmock.AnyArg()
			}
			args[37] = want
			mock.ExpectQuery("INSERT").WithArgs(args...).WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
			job, err := createBatchImageJobWithSQL(context.Background(), db, service.CreateBatchImageJobParams{TokenUsageMultiplier: multiplier})
			require.NoError(t, err)
			require.Equal(t, want, job.TokenUsageMultiplier)
			require.NoError(t, mock.ExpectationsWereMet())
			require.Len(t, captured, 1)
			shape := regexp.MustCompile(`(?s)INSERT INTO batch_image_jobs \((.*?)\) VALUES \((.*?)\)`).FindStringSubmatch(captured[0])
			require.Len(t, shape, 3)
			insertColumns := strings.Split(shape[1], ",")
			require.Len(t, insertColumns, 38)
			require.Equal(t, "token_usage_multiplier", strings.TrimSpace(insertColumns[37]))
			require.Len(t, usageLogPlaceholderRe.FindAllString(shape[2], -1), 38)
		})
	}
}
