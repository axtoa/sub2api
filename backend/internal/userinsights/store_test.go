package userinsights

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestFilterValidate(t *testing.T) {
	require.NoError(t, (Filter{Page: 1, Size: 20, Segment: "all"}).Validate())
	require.NoError(t, (Filter{Page: 1, Size: 20, Segment: "redeemed"}).Validate())
	require.NoError(t, (Filter{Page: 2, Size: 50, Segment: "inactive30"}).Validate())
	for _, filter := range []Filter{
		{Page: 0, Size: 20, Segment: "redeemed"},
		{Page: 1, Size: 51, Segment: "redeemed"},
		{Page: 1, Size: 20, Segment: "unknown"},
	} {
		require.Error(t, filter.Validate())
	}
}

func TestListIncludesUsersWithoutRedemptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(value\\), 0\\)::text").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "codes", "users"}).AddRow("0", 0, 0))
	mock.ExpectQuery("LEFT JOIN redemption r").WithArgs("").
		WillReturnRows(sqlmock.NewRows([]string{"count", "amount"}).AddRow(1, "0"))
	mock.ExpectQuery("LEFT JOIN redemption r").WithArgs("", 20, 0, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "status", "balance", "redeemed", "count", "last_redeemed", "usage", "calls", "last_used"}).
			AddRow(7, "alice@example.com", "Alice", "active", "0", "0", 0, nil, "0", 0, nil))

	result, err := NewStore(db).List(context.Background(), Filter{Page: 1, Size: 20, Segment: "all"})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, "0", result.Items[0].RedeemedAmount)
	require.Zero(t, result.Items[0].RedeemCount)
	require.Nil(t, result.Items[0].LastRedeemedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSeparatesGlobalAndFilteredTotals(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(value\\), 0\\)::text").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "codes", "users"}).AddRow("100.00", 4, 3))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\), COALESCE\\(SUM\\(r.redeemed_amount\\), 0\\)::text").
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"count", "amount"}).AddRow(1, "30.00"))
	usedAt := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery("WITH redemption AS").
		WithArgs("alice", 20, 0, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "status", "balance", "redeemed", "count", "last_redeemed", "usage", "calls", "last_used"}).
			AddRow(7, "alice@example.com", "Alice", "active", "5.00", "30.00", 2, usedAt, "25.00", 3, usedAt))
	result, err := NewStore(db).List(context.Background(), Filter{Page: 1, Size: 20, Segment: "repeat", Search: "alice"})
	require.NoError(t, err)
	require.Equal(t, "100.00", result.Summary.Amount)
	require.Equal(t, "30.00", result.FilteredAmount)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, "30.00", result.Items[0].RedeemedAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListInactiveSegmentUsesThirtyDayCutoff(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(value\\), 0\\)::text").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "codes", "users"}).AddRow("0", 0, 0))
	mock.ExpectQuery("NOT EXISTS").
		WithArgs("", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count", "amount"}).AddRow(0, "0"))
	mock.ExpectQuery("WITH redemption AS").
		WithArgs("", sqlmock.AnyArg(), 20, 0, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "status", "balance", "redeemed", "count", "last_redeemed", "usage", "calls", "last_used"}))
	result, err := NewStore(db).List(context.Background(), Filter{Page: 1, Size: 20, Segment: "inactive30"})
	require.NoError(t, err)
	require.Empty(t, result.Items)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDetailMissingUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("LEFT JOIN LATERAL").WithArgs(int64(88)).WillReturnError(sql.ErrNoRows)
	_, err = NewStore(db).Detail(context.Background(), 88)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDetailKeepsRedeemedAndAdminCreditSeparate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	usedAt := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery("LEFT JOIN LATERAL").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "status", "balance", "redeemed", "count", "first", "last"}).
			AddRow(7, "alice@example.com", "Alice", "active", "5.00", "30.00", 2, usedAt, usedAt))
	mock.ExpectQuery("type = 'admin_balance'").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow("4.00"))
	mock.ExpectQuery("SELECT value::text, used_at FROM redeem_codes").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"value", "used_at"}).AddRow("20.00", usedAt))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(actual_cost\\), 0\\)::text").WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"usage", "calls", "last"}).AddRow("25.00", 3, usedAt))
	mock.ExpectQuery("FROM usage_logs WHERE user_id = \\$1").WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"name", "calls", "cost"}).AddRow("gpt-5", 3, "25.00"))

	detail, err := NewStore(db).Detail(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "30.00", detail.RedeemedAmount)
	require.Equal(t, "4.00", detail.AdminAddedAmount)
	require.Equal(t, "25.00", detail.Usage30d)
	require.Len(t, detail.Redemptions, 1)
	require.Len(t, detail.Models, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}
