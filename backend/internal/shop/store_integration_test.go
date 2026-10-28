//go:build integration

package shop

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestPostgresShopMigrationAndConcurrentOrders(t *testing.T) {
	dsn := os.Getenv("SHOP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SHOP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	// Each run uses an isolated schema, even when the test database is shared.
	schema := "shop_test_" + fmt.Sprintf("%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	defer db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	db.SetMaxOpenConns(1)
	_, err = db.ExecContext(ctx, `SET search_path TO `+schema)
	require.NoError(t, err)
	initial, err := migrations.FS.ReadFile("001_init.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(initial))
	require.NoError(t, err)
	settingsSchema, err := migrations.FS.ReadFile("005_schema_parity.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(settingsSchema))
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("242_subscription_shop.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err, "migration must be repeatable")
	switchMigration, err := migrations.FS.ReadFile("243_official_shop_enabled.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(switchMigration))
	require.NoError(t, err)
	closedMigration, err := migrations.FS.ReadFile("250_shop_entries_default_closed.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = db.ExecContext(ctx, string(closedMigration))
		require.NoError(t, err, "closing shop entries must be repeatable")
	}
	for _, key := range []string{"official_shop_enabled", "external_shop_enabled", "shop_homepage_enabled"} {
		var value string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&value))
		require.Equal(t, "false", value, key)
	}
	_, err = db.ExecContext(ctx, `UPDATE settings SET value='true' WHERE key='official_shop_enabled'`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users(id,email,password_hash,balance) VALUES(42,'member@example.com','unused',88),(43,'other@example.com','unused',99)`)
	require.NoError(t, err)
	poolURL, err := url.Parse(dsn)
	require.NoError(t, err)
	params := poolURL.Query()
	params.Set("search_path", schema)
	poolURL.RawQuery = params.Encode()
	pool, err := sql.Open("postgres", poolURL.String())
	require.NoError(t, err)
	defer pool.Close()
	pool.SetMaxOpenConns(10)
	s := &Store{DB: pool}
	enabled, err := s.Enabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	products, err := s.Products(ctx, false)
	require.NoError(t, err)
	require.Len(t, products, 6)
	require.Contains(t, products[0].PurchaseNotice, "\n")
	for _, product := range products {
		require.Nil(t, product.PriceCents)
		require.Empty(t, product.PaymentURL)
	}
	p := productFixture()
	p.ID = products[1].ID
	p.Version = products[1].Version
	configured, err := s.SaveProduct(ctx, p)
	require.NoError(t, err)
	req := createFixture()
	req.ProductID = configured.ID
	req.ProductVersion = configured.Version
	// Requests use multiple connections on production; the advisory lock also
	// protects concurrent clients, not just the unique request-key constraint.
	var wg sync.WaitGroup
	results := make(chan *Order, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); o, e := s.Create(ctx, 42, req); results <- o; failures <- e }()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	var first *Order
	for order := range results {
		if first == nil {
			first = order
		}
		require.Equal(t, first.ID, order.ID)
	}
	list, total, err := s.Orders(ctx, OrderFilter{UserID: 42, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.EqualValues(t, 1, total)
	_, err = s.Get(ctx, first.ID, 43, false)
	require.Equal(t, 404, appErrors.Code(err))
	_, err = s.Act(ctx, first.ID, 43, false, OrderAction{Action: "cancel", Version: 1})
	require.Equal(t, 404, appErrors.Code(err))
	otherKey := req
	otherKey.RequestKey = uuid.NewString()
	_, err = s.Create(ctx, 42, otherKey)
	require.Equal(t, 409, appErrors.Code(err))
	// Closing cannot report success while a new-order transaction holds the switch.
	guard, err := pool.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer guard.Rollback()
	require.NoError(t, guard.QueryRowContext(ctx, enabledForShareSQL, "official_shop_enabled").Scan(&enabled))
	require.True(t, enabled)
	blockedCtx, stopBlocked := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err = pool.ExecContext(blockedCtx, `UPDATE settings SET value='false' WHERE key='official_shop_enabled'`)
	stopBlocked()
	require.Error(t, err, "closing must wait for the shared new-order lock")
	require.NoError(t, guard.Rollback())
	_, err = pool.ExecContext(ctx, `UPDATE settings SET value='false' WHERE key='official_shop_enabled'`)
	require.NoError(t, err)
	_, err = pool.ExecContext(ctx, string(switchMigration))
	require.NoError(t, err, "rerunning migration must not reopen purchasing")
	enabled, err = s.Enabled(ctx)
	require.NoError(t, err)
	require.False(t, enabled)
	_, err = s.Create(ctx, 42, otherKey)
	require.Equal(t, "SHOP_DISABLED", appErrors.Reason(err))
	replay, err := s.Create(ctx, 42, req)
	require.NoError(t, err, "an existing idempotent request must still resolve while closed")
	require.Equal(t, first.ID, replay.ID)
	list, total, err = s.Orders(ctx, OrderFilter{UserID: 42, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.EqualValues(t, 1, total)
	changed := *configured
	*changed.PriceCents = 15000
	_, err = s.SaveProduct(ctx, changed)
	require.NoError(t, err)
	stored, err := s.Get(ctx, first.ID, 42, false)
	require.NoError(t, err)
	require.EqualValues(t, 12300, stored.AmountCents)
	require.EqualValues(t, 12300, *stored.Product.PriceCents)
	req.TargetEmail, req.RequestKey = "different@example.com", uuid.NewString()
	_, err = s.Create(ctx, 42, req)
	require.Equal(t, "SHOP_DISABLED", appErrors.Reason(err))
	paid, err := s.Act(ctx, first.ID, 1, true, OrderAction{Action: "confirm_paid", Version: 1, ExternalReference: "EXTERNAL-1"})
	require.NoError(t, err)
	_, err = s.Act(ctx, first.ID, 1, true, OrderAction{Action: "process", Version: 1})
	require.Equal(t, 409, appErrors.Code(err))
	processing, err := s.Act(ctx, first.ID, 1, true, OrderAction{Action: "process", Version: paid.Version})
	require.NoError(t, err)
	complete, err := s.Act(ctx, first.ID, 1, true, OrderAction{Action: "complete", Version: processing.Version, Note: "Activated"})
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", complete.FulfillmentStatus)
	_, err = pool.ExecContext(ctx, `UPDATE settings SET value='true' WHERE key='official_shop_enabled'`)
	require.NoError(t, err)
	_, err = s.Create(ctx, 42, req)
	require.Equal(t, 409, appErrors.Code(err), "an old catalogue cannot place a stale-price order after reopening")
	req.ProductVersion = changed.Version + 1
	_, err = s.Create(ctx, 42, req)
	require.NoError(t, err, "new orders resume after reopening")
	_, count, err := s.Orders(ctx, OrderFilter{Admin: true, Query: "EXTERNAL-1", Payment: "PAID", Fulfillment: "COMPLETED", Product: first.Product.ID, From: time.Now().Add(-24 * time.Hour).Format(time.DateOnly), To: time.Now().Add(24 * time.Hour).Format(time.DateOnly), Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	var balance float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=42`).Scan(&balance))
	require.Equal(t, 88.0, balance, "shop actions must not touch gateway credit")
	_, err = db.ExecContext(ctx, `UPDATE users SET deleted_at=NOW() WHERE id=42`)
	require.NoError(t, err, "shop references must not prevent existing soft deletion")
}
