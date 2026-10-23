package upstreambalance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Store struct {
	DB       *sql.DB
	Location *time.Location
}
type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
type scanner interface{ Scan(...any) error }

const walletColumns = `id,config,version,observation,source_identity,status,last_error,last_attempt_at,next_query_at,failures`

func scanWallet(row scanner) (Wallet, error) {
	var w Wallet
	var c, o []byte
	e := row.Scan(&w.ID, &c, &w.Version, &o, &w.Identity, &w.Status, &w.LastError, &w.LastAttemptAt, &w.NextQueryAt, &w.Failures)
	if e != nil {
		return w, e
	}
	if e = json.Unmarshal(c, &w.Config); e != nil {
		return w, e
	}
	if len(o) > 0 {
		e = json.Unmarshal(o, &w.Observation)
	}
	w.Accounts = []AccountOption{}
	w.TodayCost, w.WeekCost, w.MonthCost, w.SinceBaselineCost = "0.000000", "0.000000", "0.000000", "0.000000"
	return w, e
}
func (s *Store) Get(ctx context.Context, id string) (Wallet, error) {
	return scanWallet(s.DB.QueryRowContext(ctx, `SELECT `+walletColumns+` FROM upstream_wallets WHERE id=$1`, id))
}
func (s *Store) Options(ctx context.Context) ([]AccountOption, error) {
	return options(ctx, s.DB)
}
func options(ctx context.Context, q querier) ([]AccountOption, error) {
	rows, e := q.QueryContext(ctx, `SELECT a.id,a.name,a.platform,a.type,a.status,COALESCE(l.wallet_id::text,'') FROM accounts a LEFT JOIN upstream_wallet_accounts l ON l.account_id=a.id WHERE a.deleted_at IS NULL ORDER BY a.name,a.id`)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	items := []AccountOption{}
	for rows.Next() {
		var a AccountOption
		if e = rows.Scan(&a.ID, &a.Name, &a.Platform, &a.Type, &a.Status, &a.WalletID); e != nil {
			return nil, e
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
func (s *Store) List(ctx context.Context, now time.Time) ([]Wallet, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	// Config, membership and consumption must come from one consistent snapshot.
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	rows, e := tx.QueryContext(ctx, `SELECT `+walletColumns+` FROM upstream_wallets ORDER BY created_at,id`)
	if e != nil {
		return nil, e
	}
	items := []Wallet{}
	indices := map[string]int{}
	for rows.Next() {
		w, err := scanWallet(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		indices[w.ID] = len(items)
		items = append(items, w)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil || len(items) == 0 {
		return items, e
	}
	options, e := options(ctx, tx)
	if e != nil {
		return nil, e
	}
	for _, a := range options {
		if i, ok := indices[a.WalletID]; ok {
			a.TodayCost, a.WeekCost, a.MonthCost = "0.000000", "0.000000", "0.000000"
			items[i].Accounts = append(items[i].Accounts, a)
		}
	}
	for i := range items {
		items[i].UsageIncomplete = len(items[i].Accounts) != len(items[i].AccountIDs) || (items[i].Mode == "manual" && items[i].EstimateEnabled && items[i].Observation != nil && now.Sub(items[i].Observation.At) > 90*24*time.Hour)
	}
	local := now.In(s.Location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.Location)
	week := now.Add(-7 * 24 * time.Hour)
	month := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, s.Location)
	// Read existing per-request account-cost snapshots. Never write usage or
	// substitute actual_cost (which is the amount billed to our customer).
	rows, e = tx.QueryContext(ctx, `SELECT l.wallet_id,l.account_id,
      COALESCE(SUM(COALESCE(u.account_stats_cost,u.total_cost)::numeric * COALESCE(u.account_rate_multiplier,1)::numeric) FILTER(WHERE u.created_at >= $1),0)::text,
      COALESCE(SUM(COALESCE(u.account_stats_cost,u.total_cost)::numeric * COALESCE(u.account_rate_multiplier,1)::numeric) FILTER(WHERE u.created_at >= $2),0)::text,
      COALESCE(SUM(COALESCE(u.account_stats_cost,u.total_cost)::numeric * COALESCE(u.account_rate_multiplier,1)::numeric) FILTER(WHERE u.created_at >= $3),0)::text,
      COALESCE(SUM(COALESCE(u.account_stats_cost,u.total_cost)::numeric * COALESCE(u.account_rate_multiplier,1)::numeric) FILTER(WHERE u.created_at >= (w.observation->>'at')::timestamptz),0)::text
      FROM upstream_wallet_accounts l JOIN upstream_wallets w ON w.id=l.wallet_id JOIN usage_logs u ON u.account_id=l.account_id
	  WHERE u.created_at >= GREATEST($4::timestamptz - INTERVAL '90 days', LEAST($1,$2,$3,CASE WHEN w.config->>'mode'='manual' AND w.config->>'estimate_enabled'='true' THEN COALESCE((w.observation->>'at')::timestamptz,$1) ELSE $1 END)) AND u.created_at <= $4
      GROUP BY l.wallet_id,l.account_id`, today, week, month, now)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	sums := map[string][4]*big.Rat{}
	for rows.Next() {
		var id, a, b, c, d string
		var accountID int64
		if e = rows.Scan(&id, &accountID, &a, &b, &c, &d); e != nil {
			return nil, e
		}
		if i, ok := indices[id]; ok {
			total, exists := sums[id]
			if !exists {
				total = [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
			}
			for n, value := range []string{a, b, c, d} {
				total[n].Add(total[n], numeric(value))
			}
			sums[id] = total
			for j := range items[i].Accounts {
				if items[i].Accounts[j].ID == accountID {
					items[i].Accounts[j].TodayCost, items[i].Accounts[j].WeekCost, items[i].Accounts[j].MonthCost = numeric(a).FloatString(6), numeric(b).FloatString(6), numeric(c).FloatString(6)
				}
			}
		}
	}
	for id, total := range sums {
		i := indices[id]
		items[i].TodayCost, items[i].WeekCost, items[i].MonthCost, items[i].SinceBaselineCost = total[0].FloatString(6), total[1].FloatString(6), total[2].FloatString(6), total[3].FloatString(6)
	}
	return items, rows.Err()
}

// Idempotency and the response commit together, including when the first HTTP
// response is lost. Changed payloads cannot reuse a previously consumed key.
func mutation[T any](ctx context.Context, s *Store, key string, body any, fn func(*sql.Tx) (T, error)) (T, error) {
	var zero T
	if _, e := uuid.Parse(key); e != nil {
		return zero, invalid("请求编号无效")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return zero, e
	}
	defer func() { _ = tx.Rollback() }()
	hash := fingerprint(body)
	res, e := tx.ExecContext(ctx, `INSERT INTO upstream_wallet_operations(id,request_hash) VALUES($1,$2) ON CONFLICT DO NOTHING`, key, hash)
	if e != nil {
		return zero, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return zero, e
	}
	if n == 0 {
		var old string
		var data []byte
		if e = tx.QueryRowContext(ctx, `SELECT request_hash,response FROM upstream_wallet_operations WHERE id=$1`, key).Scan(&old, &data); e != nil {
			return zero, e
		}
		if hash != old {
			return zero, conflict("相同请求编号不能提交不同内容")
		}
		e = json.Unmarshal(data, &zero)
		return zero, e
	}
	result, e := fn(tx)
	if e != nil {
		return zero, e
	}
	b, e := json.Marshal(result)
	if e != nil {
		return zero, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE upstream_wallet_operations SET response=$2 WHERE id=$1`, key, b); e != nil {
		return zero, e
	}
	return result, tx.Commit()
}
func event(ctx context.Context, tx *sql.Tx, id string, actor int64, action string, detail any) error {
	b, e := json.Marshal(detail)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO upstream_wallet_events(wallet_id,actor_id,action,detail) VALUES($1,$2,$3,$4)`, id, actor, action, b)
	return e
}
func (s *Store) Save(ctx context.Context, id string, in Input, actor int64) (Wallet, error) {
	if e := in.Config.validate(); e != nil {
		return Wallet{}, e
	}
	if in.InitialBalance != nil {
		v, e := amount(*in.InitialBalance, true)
		if e != nil {
			return Wallet{}, e
		}
		in.InitialBalance = &v
	}
	return mutation(ctx, s, in.RequestKey, []any{"save", id, actor, in}, func(tx *sql.Tx) (Wallet, error) {
		var before *Wallet
		var w Wallet
		if id != "" {
			old, e := scanWallet(tx.QueryRowContext(ctx, `SELECT `+walletColumns+` FROM upstream_wallets WHERE id=$1 FOR UPDATE`, id))
			if e != nil {
				return w, e
			}
			if old.Version != in.Version {
				return w, conflict("钱包已更新，请刷新")
			}
			w = old
			before = &old
		} else {
			if in.Version != 0 {
				return w, conflict("钱包版本无效")
			}
			id = uuid.NewString()
			w.ID = id
		}
		// Lock linked account rows while validating existence. Mapping is unique
		// globally, preventing accidental duplication of the same local account.
		rows, e := tx.QueryContext(ctx, `SELECT id,type FROM accounts WHERE id=ANY($1) AND deleted_at IS NULL FOR SHARE`, pq.Array(in.AccountIDs))
		if e != nil {
			return w, e
		}
		found := 0
		queryValid := in.Mode != "sub2api"
		for rows.Next() {
			var a int64
			var kind string
			if e = rows.Scan(&a, &kind); e != nil {
				_ = rows.Close()
				return w, e
			}
			found++
			if in.QueryAccountID != nil && a == *in.QueryAccountID && kind == "apikey" {
				queryValid = true
			}
		}
		e = rows.Err()
		_ = rows.Close()
		if e != nil {
			return w, e
		}
		if found != len(in.AccountIDs) || !queryValid {
			return w, invalid("关联账号不存在，或查询来源不是 API Key 账号")
		}
		reset := before == nil || before.Mode != in.Mode || before.Currency != in.Currency || !reflect.DeepEqual(before.AccountIDs, in.AccountIDs) || !reflect.DeepEqual(before.QueryAccountID, in.QueryAccountID)
		if in.Mode == "manual" && (reset || w.Observation == nil) && in.InitialBalance == nil {
			return w, invalid("新建、切换币种或关联账号后，请重新校准手动余额")
		}
		if in.Mode != "manual" && in.InitialBalance != nil {
			return w, invalid("自动查询钱包不能手工覆盖实查余额")
		}
		w.Config = in.Config
		w.Version++
		w.NextQueryAt = time.Now().UTC()
		if reset {
			w.Observation = nil
			w.Identity = ""
			w.Status = "never"
			w.LastError = ""
			w.LastAttemptAt = nil
			w.Failures = 0
		}
		if in.InitialBalance != nil {
			w.Observation = &Observation{Amount: *in.InitialBalance, Currency: w.Currency, Scope: "manual", At: time.Now().UTC()}
			w.Status = "manual"
		}
		c, _ := json.Marshal(w.Config)
		var o any
		if w.Observation != nil {
			o, _ = json.Marshal(w.Observation)
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO upstream_wallets(id,config,version,observation,source_identity,status,last_error,last_attempt_at,next_query_at,failures) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT(id) DO UPDATE SET config=EXCLUDED.config,version=EXCLUDED.version,observation=EXCLUDED.observation,source_identity=EXCLUDED.source_identity,status=EXCLUDED.status,last_error=EXCLUDED.last_error,last_attempt_at=EXCLUDED.last_attempt_at,next_query_at=EXCLUDED.next_query_at,failures=EXCLUDED.failures,lease_id=NULL,lease_until=NULL`, w.ID, c, w.Version, o, w.Identity, w.Status, w.LastError, w.LastAttemptAt, w.NextQueryAt, w.Failures)
		if e != nil {
			return w, e
		}
		if _, e = tx.ExecContext(ctx, `DELETE FROM upstream_wallet_accounts WHERE wallet_id=$1`, w.ID); e != nil {
			return w, e
		}
		for _, a := range w.AccountIDs {
			if _, e = tx.ExecContext(ctx, `INSERT INTO upstream_wallet_accounts(account_id,wallet_id) VALUES($1,$2)`, a, w.ID); e != nil {
				var pe *pq.Error
				if errors.As(e, &pe) && pe.Code == "23505" {
					return w, conflict("账号已关联其他钱包，请先解除原关联")
				}
				return w, e
			}
		}
		e = event(ctx, tx, w.ID, actor, "save", map[string]any{"before": before, "after": w})
		return w, e
	})
}
func (s *Store) Calibrate(ctx context.Context, id string, in Calibration, actor int64) (Wallet, error) {
	v, e := amount(in.Amount, true)
	if e != nil {
		return Wallet{}, e
	}
	in.Amount = v
	return mutation(ctx, s, in.RequestKey, []any{"calibrate", id, actor, in}, func(tx *sql.Tx) (Wallet, error) {
		w, e := scanWallet(tx.QueryRowContext(ctx, `SELECT `+walletColumns+` FROM upstream_wallets WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return w, e
		}
		if w.Version != in.Version {
			return w, conflict("钱包已更新，请刷新")
		}
		if w.Mode != "manual" {
			return w, invalid("只能校准手动钱包")
		}
		before := w.Observation
		w.Observation = &Observation{Amount: v, Currency: w.Currency, Scope: "manual", At: time.Now().UTC()}
		w.Version++
		w.Status = "manual"
		b, _ := json.Marshal(w.Observation)
		if _, e = tx.ExecContext(ctx, `UPDATE upstream_wallets SET observation=$2,version=$3,status='manual' WHERE id=$1`, id, b, w.Version); e != nil {
			return w, e
		}
		e = event(ctx, tx, id, actor, "calibrate", map[string]any{"before": before, "after": w.Observation})
		return w, e
	})
}
func (s *Store) History(ctx context.Context, id string) ([]Event, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT actor_id,action,detail,created_at FROM upstream_wallet_events WHERE wallet_id=$1 ORDER BY id DESC LIMIT 50`, id)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	items := []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ActorID, &v.Action, &v.Detail, &v.At); e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (s *Store) Settings(ctx context.Context) (Settings, error) {
	var v Settings
	var b []byte
	e := s.DB.QueryRowContext(ctx, `SELECT email_enabled,recipients,version FROM upstream_wallet_settings WHERE id`).Scan(&v.EmailEnabled, &b, &v.Version)
	if e == nil {
		e = json.Unmarshal(b, &v.Recipients)
	}
	return v, e
}
func (s *Store) SaveSettings(ctx context.Context, v Settings) (Settings, error) {
	if e := v.validate(); e != nil {
		return v, e
	}
	b, _ := json.Marshal(v.Recipients)
	e := s.DB.QueryRowContext(ctx, `UPDATE upstream_wallet_settings SET email_enabled=$1,recipients=$2,version=version+1 WHERE id AND version=$3 RETURNING version`, v.EmailEnabled, b, v.Version).Scan(&v.Version)
	if errors.Is(e, sql.ErrNoRows) {
		e = conflict("设置已更新，请重新打开")
	}
	return v, e
}
func (s *Store) Deliveries(ctx context.Context) ([]Delivery, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT day::text,recipient,status FROM upstream_wallet_deliveries ORDER BY day DESC,recipient LIMIT 20`)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	items := []Delivery{}
	for rows.Next() {
		var d Delivery
		if e = rows.Scan(&d.Day, &d.Recipient, &d.Status); e != nil {
			return nil, e
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
