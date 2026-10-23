package ledger

import (
	"bytes"
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"sync"
	"time"
)

type Mailer interface {
	SendEmail(context.Context, string, string, string) error
}
type Runner struct {
	Store    *Store
	Mailer   Mailer
	Location *time.Location
	cancel   context.CancelFunc
	done     chan struct{}
	start    sync.Once
	stop     sync.Once
}

func NewRunner(db *sql.DB, mailer Mailer, timezone string) *Runner {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return &Runner{Store: &Store{DB: db}, Mailer: mailer, Location: loc, done: make(chan struct{})}
}
func (r *Runner) Start() {
	r.start.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel
		go func() {
			defer close(r.done)
			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for {
				if err := r.Tick(ctx, time.Now()); err != nil && ctx.Err() == nil {
					slog.Warn("ledger reminder cycle failed")
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
func (r *Runner) Stop() {
	r.stop.Do(func() {
		if r.cancel != nil {
			r.cancel()
			<-r.done
		}
	})
}

type reminder struct {
	Resource
	Threshold int
}
type Delivery struct {
	Day       string `json:"day"`
	Recipient string `json:"recipient"`
	Status    string `json:"status"`
}

func (s *Store) Deliveries(ctx context.Context) ([]Delivery, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT day::text,recipient,status FROM ledger_email_deliveries ORDER BY day DESC,recipient LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Delivery{}
	for rows.Next() {
		var d Delivery
		if err = rows.Scan(&d.Day, &d.Recipient, &d.Status); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// Claim before contacting SMTP. A crashed/uncertain delivery is never replayed
// the same day; the next day retries thresholds not confirmed as sent.
func (s *Store) claim(ctx context.Context, now time.Time, loc *time.Location, cfg Settings, to string) ([]reminder, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	day := now.In(loc).Format(time.DateOnly)
	res, err := tx.ExecContext(ctx, `INSERT INTO ledger_email_deliveries(day,recipient) VALUES($1,$2) ON CONFLICT DO NOTHING`, day, to)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+resourceColumns+` FROM ledger_resources WHERE active ORDER BY expires_at,id`)
	if err != nil {
		return nil, err
	}
	items := []reminder{}
	for rows.Next() {
		resource, e := scanResource(rows)
		if e != nil {
			_ = rows.Close()
			return nil, e
		}
		if t, ok := thresholdFor(resource.ExpiresAt, now, cfg.ReminderDays, loc); ok {
			items = append(items, reminder{resource, t})
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	unsent := []reminder{}
	for _, item := range items {
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_reminders_sent WHERE resource_id=$1 AND expires_at=$2 AND threshold=$3 AND recipient=$4)`, item.ID, item.ExpiresAt, item.Threshold, to).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			unsent = append(unsent, item)
		}
	}
	// No claim is retained for empty digests, allowing new resources later today.
	if len(unsent) == 0 {
		return nil, nil
	}
	return unsent, tx.Commit()
}
func (s *Store) complete(ctx context.Context, now time.Time, loc *time.Location, to string, items []reminder, sent bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	status := "failed"
	if sent {
		status = "sent"
		for _, item := range items {
			if _, err = tx.ExecContext(ctx, `INSERT INTO ledger_reminders_sent(resource_id,expires_at,threshold,recipient) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, item.ID, item.ExpiresAt, item.Threshold, to); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE ledger_email_deliveries SET status=$1 WHERE day=$2 AND recipient=$3`, status, now.In(loc).Format(time.DateOnly), to); err != nil {
		return err
	}
	return tx.Commit()
}

var digestTemplate = template.Must(template.New("ledger").Parse(`<!doctype html><html lang="zh"><body><h2>Best AI · 资源续费提醒</h2><p>请到控制台「小账本 → 续费提醒」核对并手动续费。不会自动购买或扣款。</p><table cellpadding="8"><tr><th>资源</th><th>服务商</th><th>到期时间</th><th>状态</th></tr>{{range .}}<tr><td>{{.Name}}</td><td>{{.Provider}}</td><td>{{.Expiry}}</td><td>{{.Status}}</td></tr>{{end}}</table></body></html>`))

func renderDigest(items []reminder, loc *time.Location) (string, error) {
	type row struct{ Name, Provider, Expiry, Status string }
	rows := []row{}
	for _, item := range items {
		status := "即将到期"
		if item.Threshold == 0 {
			status = "已到期"
		}
		rows = append(rows, row{item.Name, item.Provider, item.ExpiresAt.In(loc).Format("2006-01-02 15:04 MST"), status})
	}
	var buf bytes.Buffer
	err := digestTemplate.Execute(&buf, rows)
	return buf.String(), err
}
func (r *Runner) Tick(ctx context.Context, now time.Time) error {
	if now.In(r.Location).Hour() < 9 || r.Mailer == nil {
		return nil
	}
	cfg, err := r.Store.Settings(ctx)
	if err != nil || !cfg.EmailEnabled {
		return err
	}
	for _, to := range cfg.Recipients {
		items, err := r.Store.claim(ctx, now, r.Location, cfg, to)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			continue
		}
		body, err := renderDigest(items, r.Location)
		if err != nil {
			return err
		}
		sendCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		sendErr := r.Mailer.SendEmail(sendCtx, to, "Best AI · 资源续费提醒", body)
		cancel()
		if err = r.Store.complete(ctx, now, r.Location, to, items, sendErr == nil); err != nil {
			return err
		}
		if sendErr != nil {
			slog.Warn("ledger reminder mail failed; inspect SMTP settings")
		}
	}
	return nil
}
