//go:build integration

package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("LEDGER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LEDGER_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("ledger_test_%d", time.Now().UnixNano())
	_, err = db.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, err)
		require.NoError(t, db.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pool, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	pool.SetMaxOpenConns(12)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	migration, err := migrations.FS.ReadFile("246_admin_ledger.sql")
	require.NoError(t, err)
	_, err = pool.Exec(string(migration))
	require.NoError(t, err)
	return pool
}
func input(kind, amount, currency string) EntryInput {
	return EntryInput{Kind: kind, Amount: amount, Currency: currency, Category: "服务器", OccurredOn: "2026-10-23", RequestKey: uuid.NewString()}
}
func resourceInput() ResourceInput {
	return ResourceInput{Name: "example.com", Kind: "domain", Currency: "USD", ExpiresAt: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC), Active: true, RequestKey: uuid.NewString()}
}
func allFilter() Filter { return Filter{Status: "all", Page: 1} }
func TestPostgresMoneyIdempotencyAndAudit(t *testing.T) {
	s := &Store{DB: testDB(t)}
	ctx := context.Background()
	in := input("income", "100.123456", "CNY")
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); e, err := s.SaveEntry(ctx, "", in, 1); ids <- e.ID; errs <- err }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	unique := map[string]bool{}
	for id := range ids {
		unique[id] = true
	}
	require.Len(t, unique, 1)
	changed := in
	changed.Amount = "200"
	_, err := s.SaveEntry(ctx, "", changed, 1)
	require.Error(t, err)
	_, err = s.SaveEntry(ctx, "", input("expense", "10.000001", "CNY"), 1)
	require.NoError(t, err)
	_, err = s.SaveEntry(ctx, "", input("gift", "50", "CNY"), 1)
	require.NoError(t, err)
	_, err = s.SaveEntry(ctx, "", input("gift", "1000", "CREDIT"), 1)
	require.NoError(t, err)
	_, err = s.SaveEntry(ctx, "", input("income", "1", "USD"), 1)
	require.NoError(t, err)
	list, err := s.Entries(ctx, allFilter(), false)
	require.NoError(t, err)
	require.Equal(t, 5, list.Total)
	require.Len(t, list.Totals, 3)
	for _, total := range list.Totals {
		if total.Currency == "CNY" {
			require.Equal(t, "90.123455", total.Net)
			require.Equal(t, "50.000000", total.Gift)
		}
	}
	entry := list.Items[0]
	edit := input(entry.Kind, "5", entry.Currency)
	edit.Version = entry.Version
	updated, err := s.SaveEntry(ctx, entry.ID, edit, 2)
	require.NoError(t, err)
	require.Equal(t, entry.Version+1, updated.Version)
	edit.RequestKey = uuid.NewString()
	_, err = s.SaveEntry(ctx, entry.ID, edit, 3)
	require.Error(t, err)
	void := VersionInput{updated.Version, uuid.NewString()}
	voided, err := s.VoidEntry(ctx, entry.ID, void, 2)
	require.NoError(t, err)
	require.True(t, voided.Voided)
	replay, err := s.VoidEntry(ctx, entry.ID, void, 2)
	require.NoError(t, err)
	require.Equal(t, voided, replay)
	events, err := s.History(ctx, entry.ID)
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.Equal(t, "entry_voided", events[0].Action)
	f := allFilter()
	f.Status = "active"
	active, err := s.Entries(ctx, f, false)
	require.NoError(t, err)
	require.Equal(t, 4, active.Total)
	f.Month = "2026-09"
	empty, err := s.Entries(ctx, f, false)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
}
func TestPostgresAtomicRenewAndResourceCreation(t *testing.T) {
	s := &Store{DB: testDB(t)}
	ctx := context.Background()
	in := input("expense", "10", "USD")
	r := resourceInput()
	in.NewResource = &r
	e, err := s.SaveEntry(ctx, "", in, 1)
	require.NoError(t, err)
	require.NotNil(t, e.ResourceID)
	resources, err := s.Resources(ctx)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	rsrc := resources[0]
	renew := RenewalInput{Version: rsrc.Version, ExpiresAt: rsrc.ExpiresAt.AddDate(1, 0, 0), RecordExpense: true, Amount: "12.5", Currency: "USD", OccurredOn: "2026-10-23", RequestKey: uuid.NewString()}
	result, err := s.Renew(ctx, rsrc.ID, renew, 1)
	require.NoError(t, err)
	require.NotNil(t, result.Entry)
	replay, err := s.Renew(ctx, rsrc.ID, renew, 1)
	require.NoError(t, err)
	require.Equal(t, result.Entry.ID, replay.Entry.ID)
	dateOnly := RenewalInput{Version: result.Resource.Version, ExpiresAt: result.Resource.ExpiresAt.AddDate(1, 0, 0), RequestKey: uuid.NewString()}
	result, err = s.Renew(ctx, rsrc.ID, dateOnly, 1)
	require.NoError(t, err)
	require.Nil(t, result.Entry)
	list, err := s.Entries(ctx, allFilter(), false)
	require.NoError(t, err)
	require.Equal(t, 2, list.Total)
	dateOnly.Version = result.Resource.Version
	dateOnly.RequestKey = uuid.NewString()
	_, err = s.Renew(ctx, rsrc.ID, dateOnly, 1)
	require.Error(t, err)
	// Force an audit failure after resource insertion: both effects must roll back.
	_, err = s.DB.Exec(`ALTER TABLE ledger_events ADD CONSTRAINT test_actor CHECK(actor_id>0)`)
	require.NoError(t, err)
	in = input("expense", "3", "USD")
	in.NewResource = &r
	_, err = s.SaveEntry(ctx, "", in, -1)
	require.Error(t, err)
	resources, err = s.Resources(ctx)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	// Also force failure after changing expiry, before creating its expense.
	_, err = s.DB.Exec(`ALTER TABLE ledger_entries ADD CONSTRAINT test_cost CHECK(amount<100)`)
	require.NoError(t, err)
	failRenew := RenewalInput{Version: result.Resource.Version, ExpiresAt: result.Resource.ExpiresAt.AddDate(1, 0, 0), RecordExpense: true, Amount: "101", Currency: "USD", OccurredOn: "2026-10-23", RequestKey: uuid.NewString()}
	_, err = s.Renew(ctx, rsrc.ID, failRenew, 1)
	require.Error(t, err)
	resources, err = s.Resources(ctx)
	require.NoError(t, err)
	require.True(t, resources[0].ExpiresAt.Equal(result.Resource.ExpiresAt))
	require.Equal(t, result.Resource.Version, resources[0].Version)
}

type testMailer struct {
	calls atomic.Int32
	fail  bool
}

func (m *testMailer) SendEmail(context.Context, string, string, string) error {
	m.calls.Add(1)
	if m.fail {
		return errors.New("private SMTP detail")
	}
	return nil
}
func TestPostgresReminderClaimsAcrossReplicasAndFailures(t *testing.T) {
	s := &Store{DB: testDB(t)}
	ctx := context.Background()
	mailer := &testMailer{}
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	in := resourceInput()
	in.ExpiresAt = now.AddDate(0, 0, 7)
	resource, err := s.SaveResource(ctx, "", in, 1)
	require.NoError(t, err)
	cfg, err := s.Settings(ctx)
	require.NoError(t, err)
	require.False(t, cfg.EmailEnabled)
	runner := &Runner{Store: s, Mailer: mailer, Location: time.UTC}
	require.NoError(t, runner.Tick(ctx, now))
	require.EqualValues(t, 0, mailer.calls.Load())
	cfg.EmailEnabled = true
	cfg.Recipients = []string{"admin@example.com"}
	_, err = s.SaveSettings(ctx, cfg, 1)
	require.NoError(t, err)
	require.NoError(t, runner.Tick(ctx, now.Add(-2*time.Hour)))
	require.EqualValues(t, 0, mailer.calls.Load())
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- runner.Tick(ctx, now) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, mailer.calls.Load())
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 1)))
	require.EqualValues(t, 1, mailer.calls.Load())
	// New one-day stage sends once, then expiry sends once.
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 6)))
	require.EqualValues(t, 2, mailer.calls.Load())
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 8)))
	require.EqualValues(t, 3, mailer.calls.Load())
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 9)))
	require.EqualValues(t, 3, mailer.calls.Load())
	// A changed expiry has independent reminder state. Failed mail retries next day.
	edit := in
	edit.Version = resource.Version
	edit.RequestKey = uuid.NewString()
	edit.ExpiresAt = now.AddDate(0, 0, 11)
	_, err = s.SaveResource(ctx, resource.ID, edit, 1)
	require.NoError(t, err)
	mailer.fail = true
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 10)))
	require.EqualValues(t, 4, mailer.calls.Load())
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 10)))
	require.EqualValues(t, 4, mailer.calls.Load())
	mailer.fail = false
	require.NoError(t, runner.Tick(ctx, now.AddDate(0, 0, 11)))
	require.EqualValues(t, 5, mailer.calls.Load())
	deliveries, err := s.Deliveries(ctx)
	require.NoError(t, err)
	require.Equal(t, "sent", deliveries[0].Status)
}

func TestPostgresPaginationFiltersAndCSVEndpoint(t *testing.T) {
	s := &Store{DB: testDB(t)}
	ctx := context.Background()
	var first Entry
	for i := 0; i < 31; i++ {
		in := input("expense", "0.000001", "USD")
		in.Category = "=1+2"
		e, err := s.SaveEntry(ctx, "", in, 1)
		require.NoError(t, err)
		first = e
	}
	f := allFilter()
	f.Page = 2
	list, err := s.Entries(ctx, f, false)
	require.NoError(t, err)
	require.Equal(t, 31, list.Total)
	require.Len(t, list.Items, 1)
	f.Query = `%' OR 1=1 --`
	list, err = s.Entries(ctx, f, false)
	require.NoError(t, err)
	require.Zero(t, list.Total)
	f = allFilter()
	f.Currency = "CNY"
	list, err = s.Entries(ctx, f, false)
	require.NoError(t, err)
	require.Empty(t, list.Items)
	_, err = s.VoidEntry(ctx, first.ID, VersionInput{Version: first.Version, RequestKey: uuid.NewString()}, 1)
	require.NoError(t, err)
	f = allFilter()
	f.Status = "voided"
	list, err = s.Entries(ctx, f, false)
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	require.Equal(t, "0", list.Totals[0].Net)
	h := &Handler{Runner: &Runner{Store: s, Location: time.UTC}}
	gin.SetMode(gin.TestMode)
	call := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		c.Set(string(middleware.ContextKeyUserRole), "admin")
		c.Request = httptest.NewRequest("GET", path, nil)
		h.Export(c)
		return w
	}
	w := call("/export?month=&status=all")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	require.Contains(t, w.Body.String(), "'=1+2")
	// A large export fails before sending CSV headers or any partial content.
	_, err = s.DB.Exec(`INSERT INTO ledger_entries(id,kind,amount,currency,category,occurred_on) SELECT gen_random_uuid(),'expense',1,'USD','bulk','2026-10-23' FROM generate_series(1,10000)`)
	require.NoError(t, err)
	w = call("/export?month=&status=all")
	require.Equal(t, 400, w.Code)
	require.NotContains(t, w.Header().Get("Content-Type"), "text/csv")
	require.NotContains(t, w.Body.String(), "SELECT")
}
