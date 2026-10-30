//go:build integration

package userinsights

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCallerFilteringAndSorting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("insights_test"), tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.ExecContext(ctx, `
		CREATE TABLE users (id bigint PRIMARY KEY, email text, username text, notes text, status text, balance numeric, deleted_at timestamptz);
        CREATE TABLE redeem_codes (used_by bigint, type text, status text, value numeric, used_at timestamptz);
        CREATE TABLE usage_logs (user_id bigint, actual_cost numeric, created_at timestamptz, model text, requested_model text);
		INSERT INTO users VALUES
		    (1, 'never@example.com', 'Never', 'no calls', 'active', 100, NULL),
		    (2, 'old@example.com', 'Old', 'legacy user', 'active', 2, NULL),
		    (3, 'free@example.com', 'Free', 'zero cost call', 'active', 10, NULL),
		    (4, 'recent@example.com', 'Recent', 'priority', 'active', 30, NULL),
		    (5, 'deleted@example.com', 'Deleted', '', 'active', 50, NOW());
        INSERT INTO redeem_codes VALUES
            (1, 'balance', 'used', 100, NOW()),
            (2, 'balance', 'used', 2, NOW() - INTERVAL '40 days'),
            (3, 'balance', 'used', 10, NOW() - INTERVAL '10 days'),
            (4, 'balance', 'used', 10, NOW() - INTERVAL '2 days'),
            (4, 'balance', 'used', 20, NOW() - INTERVAL '1 day');
        INSERT INTO usage_logs VALUES
            (2, 3, NOW() - INTERVAL '40 days', 'old', NULL),
            (3, 0, NOW() - INTERVAL '1 day', 'free', NULL),
            (3, 2, NOW() - INTERVAL '31 days', 'free', NULL),
            (4, 2, NOW() - INTERVAL '2 days', 'recent', NULL),
            (4, 8, NOW() - INTERVAL '1 hour', 'recent', NULL),
            (4, 1, NOW() - INTERVAL '40 days', 'recent', NULL),
            (5, 50, NOW(), 'deleted', NULL);
    `)
	require.NoError(t, err)
	store := NewStore(db)
	filter := Filter{Page: 1, Size: 20, Segment: "all"}

	t.Run("filters never callers across all segments and totals", func(t *testing.T) {
		for _, segment := range []string{"all", "redeemed", "repeat", "inactive30"} {
			f := filter
			f.Segment = segment
			result, err := store.List(ctx, f)
			require.NoError(t, err)
			for _, user := range result.Items {
				require.NotEqualValues(t, 1, user.ID)
				require.NotEqualValues(t, 5, user.ID)
				require.Positive(t, user.TotalCalls)
			}
			if segment == "all" {
				require.EqualValues(t, 3, result.Total)
				require.Equal(t, "42", result.FilteredAmount)
				require.Equal(t, "142", result.Summary.Amount)
			}
			if segment == "inactive30" {
				require.EqualValues(t, 1, result.Total)
				require.EqualValues(t, 2, result.Items[0].ID)
				require.Zero(t, result.Items[0].Calls30d)
				require.EqualValues(t, 1, result.Items[0].TotalCalls)
			}
		}
	})

	t.Run("counts zero cost calls and lifetime calls in list and detail", func(t *testing.T) {
		f := filter
		f.Search = "free@example.com"
		result, err := store.List(ctx, f)
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		require.EqualValues(t, 1, result.Items[0].Calls30d)
		require.EqualValues(t, 2, result.Items[0].TotalCalls)
		require.Equal(t, "0", result.Items[0].Usage30d)
		detail, err := store.Detail(ctx, 3)
		require.NoError(t, err)
		require.Equal(t, result.Items[0], detail.User)
	})

	t.Run("sorts every column across pages", func(t *testing.T) {
		for _, test := range []struct {
			key  string
			asc  []int64
			desc []int64
		}{
			{"email", []int64{3, 2, 4}, []int64{4, 2, 3}},
			{"notes", []int64{2, 4, 3}, []int64{3, 4, 2}},
			{"redeemed_amount", []int64{2, 3, 4}, []int64{4, 3, 2}},
			{"redeem_count", []int64{3, 2, 4}, []int64{4, 3, 2}},
			{"last_redeemed_at", []int64{2, 3, 4}, []int64{4, 3, 2}},
			{"balance", []int64{2, 3, 4}, []int64{4, 3, 2}},
			{"usage_30d", []int64{3, 2, 4}, []int64{4, 3, 2}},
			{"calls_30d", []int64{2, 3, 4}, []int64{4, 3, 2}},
			{"total_calls", []int64{2, 3, 4}, []int64{4, 3, 2}},
			{"last_used_at", []int64{2, 3, 4}, []int64{4, 3, 2}},
		} {
			for _, direction := range []string{"asc", "desc"} {
				t.Run(test.key+"/"+direction, func(t *testing.T) {
					var ids []int64
					for page := 1; page <= 3; page++ {
						f := filter
						f.SortBy, f.SortOrder, f.Size, f.Page = test.key, direction, 1, page
						result, err := store.List(ctx, f)
						require.NoError(t, err)
						require.Len(t, result.Items, 1)
						ids = append(ids, result.Items[0].ID)
					}
					want := test.asc
					if direction == "desc" {
						want = test.desc
					}
					require.Equal(t, want, ids)
				})
			}
		}
	})

	t.Run("keeps callers without redemptions and sorts null dates last", func(t *testing.T) {
		_, err := db.ExecContext(ctx, "DELETE FROM redeem_codes WHERE used_by = 3")
		require.NoError(t, err)
		for _, direction := range []string{"asc", "desc"} {
			f := filter
			f.SortBy, f.SortOrder = "last_redeemed_at", direction
			result, err := store.List(ctx, f)
			require.NoError(t, err)
			require.Len(t, result.Items, 3)
			require.EqualValues(t, 3, result.Items[2].ID)
			require.Zero(t, result.Items[2].RedeemCount)
			require.Nil(t, result.Items[2].LastRedeemedAt)
		}
	})
}
