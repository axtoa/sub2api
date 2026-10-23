//go:build integration

package upstreambalance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/ledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("UPSTREAM_BALANCE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("UPSTREAM_BALANCE_TEST_DATABASE_URL not configured")
	}
	db, e := sql.Open("postgres", dsn)
	require.NoError(t, e)
	schema := fmt.Sprintf("wallet_test_%d", time.Now().UnixNano())
	_, e = db.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, e)
	t.Cleanup(func() {
		_, e := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, e)
		require.NoError(t, db.Close())
	})
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pool, e := sql.Open("postgres", u.String())
	require.NoError(t, e)
	pool.SetMaxOpenConns(12)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	b, e := migrations.FS.ReadFile("247_upstream_balance.sql")
	require.NoError(t, e)
	_, e = pool.Exec(string(b))
	require.NoError(t, e)
	_, e = pool.Exec(`CREATE TABLE accounts(id BIGINT PRIMARY KEY,name TEXT,platform TEXT,type TEXT,status TEXT,deleted_at TIMESTAMPTZ);
      INSERT INTO accounts VALUES(1,'Claude 主号','anthropic','apikey','active',NULL),(2,'GPT 主号','openai','apikey','active',NULL),(3,'OAuth 账号','openai','oauth','active',NULL);
      CREATE TABLE usage_logs(account_id BIGINT,created_at TIMESTAMPTZ,total_cost NUMERIC,account_stats_cost NUMERIC,account_rate_multiplier NUMERIC,actual_cost NUMERIC);`)
	require.NoError(t, e)
	return pool
}
func input() Input {
	c := exampleConfig()
	c.AutoEnabled = false
	return Input{Config: c, RequestKey: uuid.NewString()}
}
func manualInput() Input {
	in := input()
	in.Mode = "manual"
	in.EstimateEnabled = true
	v := "100.000001"
	in.InitialBalance = &v
	return in
}

type accountSource struct {
	mu sync.Mutex
	a  *service.Account
}

func (s *accountSource) GetByID(context.Context, int64) (*service.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.a == nil {
		return nil, errors.New("missing")
	}
	b, _ := json.Marshal(s.a)
	var a service.Account
	_ = json.Unmarshal(b, &a)
	return &a, nil
}
func (s *accountSource) changeKey() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.a.Credentials["api_key"] = "changed-key"
}
func engine(t *testing.T) *Engine {
	e := NewEngine(testDB(t), &accountSource{a: exampleAccount()}, nil, &config.Config{Timezone: "Asia/Shanghai"})
	return e
}
func TestPostgresWalletIdentityIdempotencyAndEstimation(t *testing.T) {
	e := engine(t)
	ctx := context.Background()
	in := manualInput()
	var wg sync.WaitGroup
	ids := make(chan string, 10)
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w, err := e.Store.Save(ctx, "", in, 42); ids <- w.ID; errs <- err }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var id string
	for v := range ids {
		if id == "" {
			id = v
		}
		require.Equal(t, id, v)
	}
	w, err := e.Store.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "100.000001", w.Observation.Amount)
	other := manualInput()
	_, err = e.Store.Save(ctx, "", other, 42)
	require.Error(t, err) // one local account cannot belong to two wallets
	_, err = e.Store.DB.Exec(`INSERT INTO usage_logs VALUES(1,NOW(),100,2,0.5,500),(2,NOW(),100,3,2,500)`)
	require.NoError(t, err)
	items, err := e.List(ctx, time.Now())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "7.000000", items[0].TodayCost)
	require.Equal(t, "93.000001", *items[0].EstimatedBalance)
	require.Len(t, items[0].Accounts, 2)
	changed := in
	changed.Name = "different"
	_, err = e.Store.Save(ctx, "", changed, 42)
	require.Error(t, err)
	edited := Input{Config: w.Config, Version: w.Version, RequestKey: uuid.NewString()}
	edited.AccountIDs = []int64{1}
	_, err = e.Store.Save(ctx, id, edited, 42)
	require.Error(t, err) // membership changes require a new baseline
	baseline := "80"
	edited.InitialBalance = &baseline
	w, err = e.Store.Save(ctx, id, edited, 42)
	require.NoError(t, err)
	require.Equal(t, "80.000000", w.Observation.Amount)
	stale := Calibration{Version: 1, Amount: "200", RequestKey: uuid.NewString()}
	_, err = e.Store.Calibrate(ctx, id, stale, 42)
	require.Error(t, err)
	calibration := Calibration{Version: w.Version, Amount: "150.123456", RequestKey: uuid.NewString()}
	w, err = e.Store.Calibrate(ctx, id, calibration, 42)
	require.NoError(t, err)
	replayed, err := e.Store.Calibrate(ctx, id, calibration, 42)
	require.NoError(t, err)
	require.Equal(t, w.Version, replayed.Version)
	history, err := e.Store.History(ctx, id)
	require.NoError(t, err)
	require.Len(t, history, 3)
	var count int
	require.NoError(t, e.Store.DB.QueryRow(`SELECT COUNT(*) FROM upstream_wallets`).Scan(&count))
	require.Equal(t, 1, count)
}
func TestPostgresQueryManualBypassFailuresAndSourceChanges(t *testing.T) {
	e := engine(t)
	ctx := context.Background()
	w, err := e.Store.Save(ctx, "", input(), 42)
	require.NoError(t, err)
	var calls atomic.Int32
	e.Probe.Send = func(*http.Request, string) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"mode":"unrestricted","unit":"USD","balance":100}`))}, nil
	}
	_, err = e.Query(ctx, w.ID, false)
	require.NoError(t, err)
	require.Zero(t, calls.Load())
	for i := 0; i < 2; i++ {
		w, err = e.Query(ctx, w.ID, true)
		require.NoError(t, err)
		require.Equal(t, "ok", w.Status)
		require.Equal(t, "100.000000", w.Observation.Amount)
	}
	require.EqualValues(t, 2, calls.Load())
	e.Probe.Send = func(*http.Request, string) (*http.Response, error) { return nil, errors.New("private-key-proxy-error") }
	w, err = e.Query(ctx, w.ID, true)
	require.NoError(t, err)
	require.Equal(t, "failed", w.Status)
	require.Equal(t, "request_failed", w.LastError)
	require.Equal(t, "100.000000", w.Observation.Amount)
	items, err := e.List(ctx, time.Now())
	require.NoError(t, err)
	require.True(t, items[0].Stale)
	require.Nil(t, items[0].DaysRemaining)
	e.Accounts.(*accountSource).changeKey()
	items, err = e.List(ctx, time.Now())
	require.NoError(t, err)
	require.Nil(t, items[0].Observation)
	require.Equal(t, "source_changed", items[0].Status)
	options, err := e.Store.Options(ctx)
	require.NoError(t, err)
	b, _ := json.Marshal(options)
	require.NotContains(t, string(b), "fake-test-key")
	require.NotContains(t, string(b), "credentials")
}
func TestPostgresQueryLeaseAndConcurrentConfigEdit(t *testing.T) {
	e := engine(t)
	ctx := context.Background()
	w, err := e.Store.Save(ctx, "", input(), 42)
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	e.Probe.Send = func(*http.Request, string) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"mode":"unrestricted","unit":"USD","balance":100}`))}, nil
	}
	done := make(chan error, 1)
	go func() { _, err := e.Query(ctx, w.ID, true); done <- err }()
	<-entered
	_, err = e.Query(ctx, w.ID, true)
	require.Error(t, err)
	edited := Input{Config: w.Config, Version: w.Version, RequestKey: uuid.NewString()}
	edited.Name = "edited during query"
	_, err = e.Store.Save(ctx, w.ID, edited, 42)
	require.NoError(t, err)
	close(release)
	require.Error(t, <-done)
	w, err = e.Store.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Nil(t, w.Observation)
	require.Equal(t, "edited during query", w.Name)
}

type fakeMailer struct {
	calls atomic.Int32
	fail  bool
}

func (m *fakeMailer) SendEmail(context.Context, string, string, string) error {
	m.calls.Add(1)
	if m.fail {
		return errors.New("SMTP failed")
	}
	return nil
}
func TestPostgresLowAlertDedupAndNoBusinessMutations(t *testing.T) {
	e := engine(t)
	ctx := context.Background()
	in := manualInput()
	low := "200"
	in.LowThreshold = &low
	_, err := e.Store.Save(ctx, "", in, 42)
	require.NoError(t, err)
	cfg, err := e.Store.Settings(ctx)
	require.NoError(t, err)
	require.False(t, cfg.EmailEnabled)
	cfg.EmailEnabled = true
	cfg.Recipients = []string{"operator@example.com"}
	_, err = e.Store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	m := &fakeMailer{}
	e.Mailer = m
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); require.NoError(t, e.SendAlerts(ctx, time.Now())) }()
	}
	wg.Wait()
	require.EqualValues(t, 1, m.calls.Load())
	m.fail = true
	require.NoError(t, e.SendAlerts(ctx, time.Now().Add(24*time.Hour)))
	require.EqualValues(t, 2, m.calls.Load())
	require.NoError(t, e.SendAlerts(ctx, time.Now().Add(24*time.Hour)))
	require.EqualValues(t, 2, m.calls.Load())
	var accounts, usage int
	require.NoError(t, e.Store.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&accounts))
	require.Equal(t, 3, accounts)
	require.NoError(t, e.Store.DB.QueryRow(`SELECT COUNT(*) FROM usage_logs`).Scan(&usage))
	require.Zero(t, usage)
}

func TestPostgresProbePanicReleasesLeaseAndOtherWalletStillWorks(t *testing.T) {
	e := engine(t)
	ctx := context.Background()
	w, err := e.Store.Save(ctx, "", input(), 42)
	require.NoError(t, err)
	second := input()
	second.AccountIDs = []int64{3}
	second.Mode = "manual"
	amount := "20"
	second.InitialBalance = &amount
	other, err := e.Store.Save(ctx, "", second, 42)
	require.NoError(t, err)
	e.Probe.Send = func(*http.Request, string) (*http.Response, error) { panic("secret-key") }
	for i := 0; i < 2; i++ {
		failed, err := e.Query(ctx, w.ID, true)
		require.NoError(t, err)
		require.Equal(t, "failed", failed.Status)
		require.Equal(t, "internal_error", failed.LastError)
	}
	_, err = e.Store.Calibrate(ctx, other.ID, Calibration{Amount: "30", Version: other.Version, RequestKey: uuid.NewString()}, 42)
	require.NoError(t, err)
	e.Probe.Send = func(*http.Request, string) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"mode":"unrestricted","unit":"USD","balance":50}`))}, nil
	}
	recovered, err := e.Query(ctx, w.ID, true)
	require.NoError(t, err)
	require.Equal(t, "ok", recovered.Status)
}

func TestPostgresBrokenBalanceTablesDoNotAffectRealLedger(t *testing.T) {
	e := engine(t)
	ctx := context.Background()
	migration, err := migrations.FS.ReadFile("246_admin_ledger.sql")
	require.NoError(t, err)
	_, err = e.Store.DB.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	// Rename only the fixture's balance table. The real ledger still reads and
	// writes its own tables after foreground AND background observation fail.
	_, err = e.Store.DB.ExecContext(ctx, `ALTER TABLE upstream_wallets RENAME TO broken_upstream_wallets`)
	require.NoError(t, err)
	_, err = e.List(ctx, time.Now())
	require.Error(t, err)
	require.Error(t, e.tickSafely(ctx, time.Now()))
	l := &ledger.Store{DB: e.Store.DB}
	entry, err := l.SaveEntry(ctx, "", ledger.EntryInput{Kind: "expense", Amount: "12.345678", Currency: "USD", Category: "服务器", OccurredOn: "2026-10-23", RequestKey: uuid.NewString()}, 42)
	require.NoError(t, err)
	require.Equal(t, "12.345678", entry.Amount)
	list, err := l.Entries(ctx, ledger.Filter{Status: "all", Page: 1}, false)
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	require.Equal(t, entry.ID, list.Items[0].ID)
}

func TestPostgresDedicatedPoolHasBudgetsAndNeverClosesPrimary(t *testing.T) {
	dsn := os.Getenv("UPSTREAM_BALANCE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("UPSTREAM_BALANCE_TEST_DATABASE_URL not configured")
	}
	primary, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer primary.Close()
	pool, err := openWalletPool(dsn)
	require.NoError(t, err)
	defer pool.Close()
	// Pool options are enforced by PostgreSQL even when a client context is lost.
	var statement, lock string
	require.NoError(t, pool.QueryRow(`SHOW statement_timeout`).Scan(&statement))
	require.NoError(t, pool.QueryRow(`SHOW lock_timeout`).Scan(&lock))
	require.Equal(t, "8s", statement)
	require.Equal(t, "2s", lock)
	require.Equal(t, 2, pool.Stats().MaxOpenConnections)
	first, err := pool.Conn(context.Background())
	require.NoError(t, err)
	second, err := pool.Conn(context.Background())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	require.Error(t, pool.PingContext(ctx))
	require.NoError(t, primary.PingContext(context.Background()))
	require.NoError(t, first.Close())
	require.NoError(t, second.Close())
	e := NewEngine(pool, nil, nil, nil)
	e.ownedDB = pool
	e.Stop()
	require.NoError(t, primary.PingContext(context.Background()))
	require.Error(t, pool.PingContext(context.Background()))
}
