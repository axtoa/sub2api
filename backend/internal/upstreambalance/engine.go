package upstreambalance

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"log/slog"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

type Mailer interface {
	SendEmail(context.Context, string, string, string) error
}
type Engine struct {
	Store       *Store
	Accounts    AccountSource
	Probe       *Probe
	Mailer      Mailer
	cancel      context.CancelFunc
	done        chan struct{}
	start, stop sync.Once
	slots       chan struct{}
	ownedDB     *sql.DB
}

func NewEngine(db *sql.DB, accounts AccountSource, mailer Mailer, cfg *config.Config) *Engine {
	tz := "UTC"
	if cfg != nil {
		tz = cfg.Timezone
	}
	loc, e := time.LoadLocation(tz)
	if e != nil {
		loc = time.UTC
	}
	return &Engine{Store: &Store{DB: db, Location: loc}, Accounts: accounts, Probe: &Probe{Config: cfg}, Mailer: mailer, done: make(chan struct{}), slots: make(chan struct{}, 3)}
}
func (e *Engine) Start() {
	e.start.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel
		go func() {
			defer close(e.done)
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				if err := e.tickSafely(ctx, time.Now()); err != nil && ctx.Err() == nil {
					slog.Warn("upstream wallet observation cycle failed")
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}
func (e *Engine) Stop() {
	e.stop.Do(func() {
		if e.cancel != nil {
			e.cancel()
			<-e.done
		}
		if e.ownedDB != nil {
			_ = e.ownedDB.Close()
		}
	})
}
func (e *Engine) List(ctx context.Context, now time.Time) ([]Wallet, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	items, err := e.Store.List(ctx, now)
	if err != nil {
		return nil, err
	}
	for i := range items {
		w := &items[i]
		if w.Mode == "sub2api" && w.Observation != nil && w.QueryAccountID != nil {
			a, err := e.Accounts.GetByID(ctx, *w.QueryAccountID)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if err != nil || sourceIdentity(a) != w.Identity {
				w.Observation = nil
				w.Status = "source_changed"
				w.LastError = "source_changed"
			}
		}
		w.decorate(now)
	}
	return items, nil
}
func (e *Engine) Query(ctx context.Context, id string, manual bool) (Wallet, error) {
	ctx, queryCancel := context.WithTimeout(ctx, 20*time.Second)
	defer queryCancel()
	// Manual requests bypass auto-enabled, interval and failure backoff, not
	// administrative authentication or bounded concurrency.
	select {
	case e.slots <- struct{}{}:
		defer func() { <-e.slots }()
	case <-ctx.Done():
		return Wallet{}, ctx.Err()
	}
	w, err := e.Store.Get(ctx, id)
	if err != nil {
		return w, err
	}
	if w.Mode != "sub2api" || w.QueryAccountID == nil {
		return w, invalid("手动钱包请使用校准余额")
	}
	if !manual && (!w.AutoEnabled || w.NextQueryAt.After(time.Now())) {
		return w, nil
	}
	lease := uuid.NewString()
	var claimed string
	err = e.Store.DB.QueryRowContext(ctx, `UPDATE upstream_wallets SET lease_id=$2,lease_until=NOW()+INTERVAL '45 seconds' WHERE id=$1 AND version=$3 AND (lease_until IS NULL OR lease_until<NOW()) AND ($4 OR next_query_at<=NOW()) RETURNING id`, id, lease, w.Version, manual).Scan(&claimed)
	if err == sql.ErrNoRows {
		return w, conflict("该钱包正在查询或配置已更新，请稍后重试")
	}
	if err != nil {
		return w, err
	}
	now := time.Now().UTC()
	status := "ok"
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	observation, identity, reason := e.observe(probeCtx, w)
	failures := 0
	delay := time.Duration(w.IntervalMinutes) * time.Minute
	if reason != "" {
		status = "failed"
		if reason == "unsupported" || reason == "no_balance" {
			status = "unsupported"
		}
		failures = w.Failures + 1
		shift := failures
		if shift > 5 {
			shift = 5
		}
		delay *= time.Duration(1 << shift)
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
	}
	var body any
	if observation != nil {
		body, _ = json.Marshal(observation)
	}
	// A cancelled browser must not leave a lease permanently claimed. Use a
	// short independent context to record the outcome; never retry model calls.
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()
	res, saveErr := e.Store.DB.ExecContext(saveCtx, `UPDATE upstream_wallets SET
      observation=CASE WHEN $4::jsonb IS NOT NULL THEN $4::jsonb WHEN source_identity<>$5 THEN NULL ELSE observation END,
      source_identity=$5,status=$6,last_error=$7,last_attempt_at=$8,next_query_at=$9,failures=$10,lease_id=NULL,lease_until=NULL
      WHERE id=$1 AND version=$2 AND lease_id=$3`, id, w.Version, lease, body, identity, status, reason, now, now.Add(delay), failures)
	if saveErr != nil {
		return w, saveErr
	}
	n, saveErr := res.RowsAffected()
	if saveErr != nil {
		return w, saveErr
	}
	if n == 0 {
		return w, conflict("查询期间钱包配置已更新，请重新查询")
	}
	result, getErr := e.Store.Get(saveCtx, id)
	return result, getErr
}
func (e *Engine) Tick(ctx context.Context, now time.Time) error {
	rows, err := e.Store.DB.QueryContext(ctx, `SELECT id FROM upstream_wallets WHERE config->>'mode'='sub2api' AND config->>'auto_enabled'='true' AND next_query_at<=$1 AND (lease_until IS NULL OR lease_until<$1) ORDER BY next_query_at LIMIT 20`, now)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = runBalanceTask(func() error { _, err := e.Query(ctx, id, false); return err })
		}(id)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return e.SendAlerts(ctx, now)
}

var alertTemplate = template.Must(template.New("wallet").Parse(`<h2>上游余额提醒</h2><p>仅供运营参考，不会自动充值或停用账号。手动和估算余额可能有偏差。</p><ul>{{range .}}<li><strong>{{.Name}}</strong>：{{if .EstimatedBalance}}{{.EstimatedBalance}}（本站估算）{{else}}{{.Observation.Amount}}{{end}} {{.Observation.Currency}} · {{.Observation.Scope}}，基准时间 {{.Observation.At}}</li>{{end}}</ul>`))

func (e *Engine) SendAlerts(ctx context.Context, now time.Time) error {
	cfg, err := e.Store.Settings(ctx)
	if err != nil || !cfg.EmailEnabled {
		return err
	}
	if e.Mailer == nil {
		return nil
	}
	wallets, err := e.List(ctx, now)
	if err != nil {
		return err
	}
	low := []Wallet{}
	for _, w := range wallets {
		if w.Low && !w.Stale && !w.UsageIncomplete && w.Observation != nil {
			low = append(low, w)
		}
	}
	if len(low) == 0 {
		return nil
	}
	var html bytes.Buffer
	if err = alertTemplate.Execute(&html, low); err != nil {
		return err
	}
	day := now.In(e.Store.Location).Format(time.DateOnly)
	for _, to := range cfg.Recipients {
		res, err := e.Store.DB.ExecContext(ctx, `INSERT INTO upstream_wallet_deliveries(day,recipient) VALUES($1,$2) ON CONFLICT DO NOTHING`, day, to)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		mailCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		sendErr := e.Mailer.SendEmail(mailCtx, to, "上游余额不足提醒", html.String())
		cancel()
		status := "sent"
		if sendErr != nil {
			status = "failed"
		}
		if _, err = e.Store.DB.ExecContext(ctx, `UPDATE upstream_wallet_deliveries SET status=$3 WHERE day=$1 AND recipient=$2`, day, to, status); err != nil {
			return err
		}
	}
	return nil
}
