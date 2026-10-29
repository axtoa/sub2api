package userinsights

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestListHandlerRejectsInvalidSort(t *testing.T) {
	for _, query := range []string{"sort_by=unknown", "sort_order=unknown"} {
		t.Run(query, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/user-insights?"+query, nil)
			(&Handler{}).List(ctx)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestListHandlerPassesSortAndPagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(value\\), 0\\)::text").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "codes", "users"}).AddRow("0", 0, 0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\), COALESCE").WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"count", "amount"}).AddRow(0, "0"))
	mock.ExpectQuery(`ORDER BY i.total_calls ASC NULLS LAST, i.id DESC LIMIT \$2 OFFSET \$3`).
		WithArgs("alice", 10, 10, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "status", "balance", "redeemed", "count", "last_redeemed", "usage", "calls", "total_calls", "last_used"}))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/user-insights?search=alice&page=2&page_size=10&sort_by=total_calls&sort_order=ASC", nil)
	NewHandler(db).List(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
