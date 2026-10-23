package upstreambalance

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// This optional observer must not borrow the gateway/ledger SQL connection
// pool for scans, transactions or background work. No startup Ping is required.
func openWalletPool(dsn string) (*sql.DB, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return nil, errors.New("upstream wallet database configuration unavailable")
		}
		q := u.Query()
		for key, value := range map[string]string{"connect_timeout": "3", "statement_timeout": "8000", "lock_timeout": "2000", "application_name": "sub2api_upstream_balance"} {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " connect_timeout=3 statement_timeout=8000 lock_timeout=2000 application_name=sub2api_upstream_balance"
	}
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, errors.New("upstream wallet database configuration unavailable")
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

func NewIsolatedEngine(accounts AccountSource, mailer Mailer, cfg *config.Config) (*Engine, error) {
	if cfg == nil {
		return nil, errors.New("upstream wallet configuration unavailable")
	}
	db, err := openWalletPool(cfg.Database.DSNWithTimezone(cfg.Timezone))
	if err != nil {
		return nil, err
	}
	e := NewEngine(db, accounts, mailer, cfg)
	e.ownedDB = db
	return e, nil
}

// Recover only at this feature's task boundaries. Do not log panic payloads:
// they may contain transport URLs, account credentials or mail settings.
func runBalanceTask(task func() error) (err error) {
	defer func() {
		if recover() != nil {
			slog.Error("upstream wallet task panic contained")
			err = errors.New("upstream wallet task unavailable")
		}
	}()
	return task()
}

func (e *Engine) tickSafely(ctx context.Context, now time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	return runBalanceTask(func() error { return e.Tick(ctx, now) })
}

func (e *Engine) observe(ctx context.Context, w Wallet) (observation *Observation, identity, reason string) {
	defer func() {
		if recover() != nil {
			observation, reason = nil, "internal_error"
			slog.Error("upstream wallet probe panic contained")
		}
	}()
	a, err := e.Accounts.GetByID(ctx, *w.QueryAccountID)
	if err != nil {
		return nil, "", "invalid_account"
	}
	identity = sourceIdentity(a)
	observation, err = e.Probe.Query(ctx, a)
	if err != nil {
		return nil, identity, err.Error()
	}
	if observation == nil {
		return nil, identity, "invalid_response"
	}
	if observation.Currency != w.Currency {
		return nil, identity, "currency_mismatch"
	}
	// A response using an old credential must never overwrite the new source.
	fresh, err := e.Accounts.GetByID(ctx, *w.QueryAccountID)
	if err != nil || sourceIdentity(fresh) != identity {
		return nil, identity, "source_changed"
	}
	return observation, identity, ""
}

// Apply only to the optional balance routes, after real administrator auth.
// Initialization/runtime failures stay within this feature, not other routes.
func (h *Handler) Isolation(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.Engine == nil {
		response.Error(c, http.StatusServiceUnavailable, "上游余额暂不可用，其他功能可继续使用")
		c.Abort()
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	defer func() {
		if recover() != nil {
			slog.Error("upstream wallet request panic contained")
			if !c.Writer.Written() {
				response.Error(c, http.StatusServiceUnavailable, "上游余额暂不可用，其他功能可继续使用")
			}
			c.Abort()
		}
	}()
	c.Next()
}
