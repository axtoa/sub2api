package quality

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store struct{ DB *sql.DB }

func (s *Store) Settings(ctx context.Context) (Settings, error) {
	d := DefaultSettings()
	raw, err := json.Marshal(d)
	if err != nil {
		return d, err
	}
	if _, err = s.DB.ExecContext(ctx, `INSERT INTO quality_monitor_settings(id,payload) VALUES(TRUE,$1) ON CONFLICT DO NOTHING`, string(raw)); err != nil {
		return d, err
	}
	err = s.DB.QueryRowContext(ctx, `SELECT payload,version FROM quality_monitor_settings WHERE id=TRUE`).Scan(&raw, &d.Version)
	if err != nil {
		return d, err
	}
	version := d.Version
	err = json.Unmarshal(raw, &d)
	d.Version = version
	return d, err
}

func (s *Store) SaveSettings(ctx context.Context, cfg Settings) (Settings, error) {
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return cfg, err
	}
	defer func() { _ = tx.Rollback() }()
	previous := cfg.Version
	cfg.Version++
	raw, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	var version int
	err = tx.QueryRowContext(ctx, `UPDATE quality_monitor_settings SET payload=$1,version=version+1,updated_at=NOW() WHERE id=TRUE AND version=$2 RETURNING version`, string(raw), previous).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, conflict("设置已被修改，请刷新后重试")
	}
	if err != nil {
		return cfg, err
	}
	// Settings changes retain today's usage. Recalculate future schedules from
	// their own last run, including individual overrides, instead of multiplying jobs.
	_, err = tx.ExecContext(ctx, `UPDATE quality_monitor_targets SET
 next_availability_at=GREATEST(NOW()+INTERVAL '5 seconds',COALESCE(last_availability_at,NOW())+COALESCE((overrides->>'availability_interval_minutes')::int,$1)*INTERVAL '1 minute'),
 next_quality_at=GREATEST(NOW()+INTERVAL '5 seconds',COALESCE(last_quality_at,NOW())+COALESCE((overrides->>'quality_interval_minutes')::int,$2)*INTERVAL '1 minute'),
 next_cache_at=GREATEST(NOW()+INTERVAL '5 seconds',COALESCE(last_cache_at,NOW())+COALESCE((overrides->>'cache_interval_minutes')::int,$3)*INTERVAL '1 minute'),retry_attempt=0`, cfg.AvailabilityIntervalMinutes, cfg.QualityIntervalMinutes, cfg.CacheIntervalMinutes)
	if err != nil {
		return cfg, err
	}
	return cfg, tx.Commit()
}

const targetSelect = `SELECT t.id,t.account_id,a.name,a.platform,a.status,t.model,t.enabled,t.overrides,t.version,
t.next_availability_at,t.next_quality_at,t.next_cache_at,t.last_availability_at,t.last_quality_at,t.last_cache_at,t.retry_attempt,COALESCE(m.enabled,TRUE)
FROM quality_monitor_targets t JOIN accounts a ON a.id=t.account_id AND a.deleted_at IS NULL
LEFT JOIN quality_monitor_accounts m ON m.account_id=t.account_id `

type scanner interface{ Scan(...any) error }

func scanTarget(row scanner) (Target, error) {
	var t Target
	var raw []byte
	err := row.Scan(&t.ID, &t.AccountID, &t.AccountName, &t.Platform, &t.AccountStatus, &t.Model, &t.Enabled, &raw, &t.Version,
		&t.NextAvailabilityAt, &t.NextQualityAt, &t.NextCacheAt, &t.LastAvailabilityAt, &t.LastQualityAt, &t.LastCacheAt, &t.RetryAttempt, &t.AccountEnabled)
	if err == nil && len(raw) > 0 {
		err = json.Unmarshal(raw, &t.Overrides)
	}
	t.Recent = []Run{}
	return t, err
}

func (s *Store) Target(ctx context.Context, id string) (Target, error) {
	return scanTarget(s.DB.QueryRowContext(ctx, targetSelect+` WHERE t.id=$1`, id))
}

func (s *Store) Targets(ctx context.Context) ([]Target, error) {
	rows, err := s.DB.QueryContext(ctx, targetSelect+` ORDER BY a.name,t.model`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Target{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Store) SaveTarget(ctx context.Context, t Target, cfg Settings) (Target, error) {
	if err := validateModel(t.Model); err != nil {
		return t, err
	}
	if t.Overrides != nil {
		if err := t.Overrides.Validate(); err != nil {
			return t, err
		}
	}
	var raw any
	if t.Overrides != nil {
		b, err := json.Marshal(t.Overrides)
		if err != nil {
			return t, err
		}
		raw = string(b)
	}
	var err error
	if t.ID == "" {
		// Serialize model additions with inventory sync across replicas.
		tx, txErr := s.DB.BeginTx(ctx, nil)
		if txErr != nil {
			return t, txErr
		}
		defer func() { _ = tx.Rollback() }()
		var settingsID bool
		if txErr = tx.QueryRowContext(ctx, `SELECT id FROM quality_monitor_settings WHERE id=TRUE FOR UPDATE`).Scan(&settingsID); txErr != nil {
			return t, txErr
		}
		t.ID = uuid.NewString()
		err = tx.QueryRowContext(ctx, `INSERT INTO quality_monitor_targets(id,account_id,model,enabled,overrides)
 VALUES($1,$2,$3,$4,$5)
 ON CONFLICT(account_id,model) DO NOTHING RETURNING version`, t.ID, t.AccountID, t.Model, t.Enabled, raw).Scan(&t.Version)
		if err == nil {
			err = tx.Commit()
		}
	} else {
		p := t.Effective(cfg)
		err = s.DB.QueryRowContext(ctx, `UPDATE quality_monitor_targets SET enabled=$1,overrides=$2,version=version+1,updated_at=NOW(),retry_attempt=0,
 next_availability_at=GREATEST(NOW()+INTERVAL '5 seconds',COALESCE(last_availability_at,NOW())+$3*INTERVAL '1 minute'),
 next_quality_at=GREATEST(NOW()+INTERVAL '5 seconds',COALESCE(last_quality_at,NOW())+$4*INTERVAL '1 minute'),
 next_cache_at=GREATEST(NOW()+INTERVAL '5 seconds',COALESCE(last_cache_at,NOW())+$5*INTERVAL '1 minute')
 WHERE id=$6 AND version=$7 AND account_id=$8 AND model=$9 RETURNING version`, t.Enabled, raw,
			p.AvailabilityIntervalMinutes, p.QualityIntervalMinutes, p.CacheIntervalMinutes, t.ID, t.Version, t.AccountID, t.Model).Scan(&t.Version)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return t, conflict("该账号模型已添加，或配置已被其他人修改，请刷新")
	}
	return t, err
}

func (s *Store) DeleteTarget(ctx context.Context, id string) error {
	// Retain history: deletion is implemented as disabling the target. A running
	// job observes this before its next request. It never deletes an account.
	_, err := s.DB.ExecContext(ctx, `UPDATE quality_monitor_targets SET enabled=FALSE,version=version+1,updated_at=NOW() WHERE id=$1`, id)
	return err
}

const runColumns = `id,target_id,kind,status,manual,verdict,profile_hash,config_snapshot,result,created_at,started_at,finished_at`

func scanRun(row scanner) (Run, error) {
	var r Run
	var configRaw, resultRaw []byte
	err := row.Scan(&r.ID, &r.TargetID, &r.Kind, &r.Status, &r.Manual, &r.Verdict, &r.ProfileHash, &configRaw, &resultRaw, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	if err == nil && len(configRaw) > 0 {
		err = json.Unmarshal(configRaw, &r.Profile)
	}
	if err == nil && len(resultRaw) > 0 {
		err = json.Unmarshal(resultRaw, &r.Result)
	}
	return r, err
}

func (s *Store) History(ctx context.Context, targetID string, limit int) ([]Run, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+runColumns+` FROM quality_monitor_runs WHERE target_id=$1 ORDER BY created_at DESC LIMIT $2`, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

func (s *Store) Queue(ctx context.Context, id, kind string, manual bool) (Run, error) {
	if !validKind(kind) {
		return Run{}, invalid("检测类型无效")
	}
	r, err := scanRun(s.DB.QueryRowContext(ctx, `INSERT INTO quality_monitor_runs(id,target_id,kind,status,manual)
 SELECT $1,$2,$3,'queued',$4 WHERE $4 OR NOT EXISTS(SELECT 1 FROM quality_monitor_runs WHERE target_id=$2 AND status IN ('queued','running'))
 ON CONFLICT DO NOTHING RETURNING `+runColumns, uuid.NewString(), id, kind, manual))
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, conflict("该账号模型已有检测排队或执行中")
	}
	return r, err
}

func (s *Store) Claim(ctx context.Context) (*Run, error) {
	// A crashed worker's budget remains reserved. Mark its result unknown;
	// never replay requests which may already have consumed upstream tokens.
	_, err := s.DB.ExecContext(ctx, `WITH interrupted AS (UPDATE quality_monitor_runs SET status='finished',verdict='unknown',
 result='{"reason":"interrupted","items":[]}'::jsonb,finished_at=NOW()
 WHERE status='running' AND lease_until<NOW() RETURNING target_id,kind,manual)
 UPDATE quality_monitor_targets t SET
 next_availability_at=CASE WHEN i.kind='availability' THEN GREATEST(t.next_availability_at,NOW()+COALESCE((t.overrides->>'availability_interval_minutes')::int,(s.payload->>'availability_interval_minutes')::int)*INTERVAL '1 minute') ELSE t.next_availability_at END,
 next_quality_at=CASE WHEN i.kind='quality' THEN GREATEST(t.next_quality_at,NOW()+COALESCE((t.overrides->>'quality_interval_minutes')::int,(s.payload->>'quality_interval_minutes')::int)*INTERVAL '1 minute') ELSE t.next_quality_at END,
 next_cache_at=CASE WHEN i.kind='cache' THEN GREATEST(t.next_cache_at,NOW()+COALESCE((t.overrides->>'cache_interval_minutes')::int,(s.payload->>'cache_interval_minutes')::int)*INTERVAL '1 minute') ELSE t.next_cache_at END,
 retry_attempt=CASE WHEN i.kind='quality' THEN 0 ELSE t.retry_attempt END
 FROM interrupted i,quality_monitor_settings s WHERE t.id=i.target_id AND s.id=TRUE AND NOT i.manual`)
	if err != nil {
		return nil, err
	}
	r, err := scanRun(s.DB.QueryRowContext(ctx, `UPDATE quality_monitor_runs SET status='running',started_at=NOW(),lease_until=NOW()+INTERVAL '3 minutes'
	 WHERE status='queued' AND id=(SELECT q.id FROM quality_monitor_runs q JOIN quality_monitor_targets t ON t.id=q.target_id
 WHERE q.status='queued' AND NOT EXISTS(SELECT 1 FROM quality_monitor_runs active WHERE active.target_id=q.target_id AND active.status='running')
 ORDER BY q.manual DESC,q.created_at,q.id FOR UPDATE OF t,q SKIP LOCKED LIMIT 1)
 RETURNING `+runColumns))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

func (s *Store) Renew(ctx context.Context, r Run, p Profile, hash string) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE quality_monitor_runs SET lease_until=NOW()+INTERVAL '3 minutes',config_snapshot=$1,profile_hash=$2 WHERE id=$3 AND status='running' AND lease_until>NOW()`, string(raw), hash, r.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n != 1 {
		return conflict("检测执行租约已过期")
	}
	return err
}

func (s *Store) Finish(ctx context.Context, r Run, next time.Time, retry, targetVersion, settingsVersion int) error {
	raw, err := json.Marshal(r.Result)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var id string
	err = tx.QueryRowContext(ctx, `UPDATE quality_monitor_runs SET status='finished',verdict=$1,result=$2,finished_at=NOW(),lease_until=NULL
 WHERE id=$3 AND status='running' AND lease_until>NOW() RETURNING target_id`, r.Verdict, string(raw), r.ID).Scan(&id)
	if err != nil {
		return err
	}
	if r.Manual {
		return tx.Commit()
	}
	column, last := "next_availability_at", "last_availability_at"
	if r.Kind == "quality" {
		column, last = "next_quality_at", "last_quality_at"
	} else if r.Kind == "cache" {
		column, last = "next_cache_at", "last_cache_at"
	}
	// Column names are selected exclusively from the constants above.
	_, err = tx.ExecContext(ctx, `UPDATE quality_monitor_targets SET `+column+`=$1,`+last+`=NOW(),retry_attempt=CASE WHEN $2='quality' THEN $3 ELSE retry_attempt END
 WHERE id=$4 AND version=$5 AND EXISTS(SELECT 1 FROM quality_monitor_settings WHERE id=TRUE AND version=$6)`, next, r.Kind, retry, id, targetVersion, settingsVersion)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Budget(ctx context.Context, cfg Settings, now time.Time, loc *time.Location) (Budget, error) {
	b := Budget{Day: now.In(loc).Format("2006-01-02"), Timezone: loc.String()}
	err := s.DB.QueryRowContext(ctx, `SELECT requests,tokens,unconfirmed_requests,manual_requests,manual_tokens,manual_unconfirmed FROM quality_monitor_daily_budget WHERE day=$1`, b.Day).Scan(&b.Requests, &b.Tokens, &b.UnconfirmedRequests, &b.ManualRequests, &b.ManualTokens, &b.ManualUnconfirmed)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	b.Limited = b.Requests >= cfg.DailyRequests || b.Tokens >= cfg.DailyTokens
	return b, err
}

func (s *Store) Reserve(ctx context.Context, day string, tokens int64) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock the current limits as well as the day row. Settings edits cannot
	// race a reservation into an outdated budget or reset spent quota.
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT payload FROM quality_monitor_settings WHERE id=TRUE FOR SHARE`).Scan(&raw); err != nil {
		return false, err
	}
	var cfg Settings
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO quality_monitor_daily_budget(day) VALUES($1) ON CONFLICT DO NOTHING`, day); err != nil {
		return false, err
	}
	var count int
	err = tx.QueryRowContext(ctx, `UPDATE quality_monitor_daily_budget SET requests=requests+1,tokens=tokens+$1,unconfirmed_requests=unconfirmed_requests+1
 WHERE day=$2 AND requests<$3 AND tokens+$1<=$4 RETURNING requests`, tokens, day, cfg.DailyRequests, cfg.DailyTokens).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) Settle(ctx context.Context, day string, reserved, actual int64, confirmed bool) error {
	return s.settle(ctx, day, reserved, actual, confirmed, false)
}

func (s *Store) ReserveManual(ctx context.Context, day string, tokens int64) (bool, error) {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO quality_monitor_daily_budget(day,manual_requests,manual_tokens,manual_unconfirmed) VALUES($1,1,$2,1)
 ON CONFLICT(day) DO UPDATE SET manual_requests=quality_monitor_daily_budget.manual_requests+1,
 manual_tokens=quality_monitor_daily_budget.manual_tokens+$2,manual_unconfirmed=quality_monitor_daily_budget.manual_unconfirmed+1`, day, tokens)
	return err == nil, err
}

func (s *Store) settle(ctx context.Context, day string, reserved, actual int64, confirmed, manual bool) error {
	if !confirmed {
		actual = max(actual, reserved)
	}
	confirmedCount := 0
	if confirmed {
		confirmedCount = 1
	}
	query := `UPDATE quality_monitor_daily_budget SET tokens=GREATEST(0,tokens+$1),unconfirmed_requests=GREATEST(0,unconfirmed_requests-$2) WHERE day=$3`
	if manual {
		query = `UPDATE quality_monitor_daily_budget SET manual_tokens=GREATEST(0,manual_tokens+$1),manual_unconfirmed=GREATEST(0,manual_unconfirmed-$2) WHERE day=$3`
	}
	_, err := s.DB.ExecContext(ctx, query, actual-reserved, confirmedCount, day)
	return err
}

func (s *Store) Metrics(ctx context.Context, t Target, since time.Time) (Metrics, error) {
	var m Metrics
	// Billing aggregates are explicitly labelled as such. They are NOT used
	// to claim that upstream prompt caching actually happened.
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*),AVG(first_token_ms),COALESCE(SUM(input_tokens),0),COALESCE(SUM(cache_read_tokens),0),MAX(created_at)
 FROM usage_logs WHERE account_id=$1 AND COALESCE(NULLIF(requested_model,''),model)=$2 AND created_at>=$3`, t.AccountID, t.Model, since).
		Scan(&m.Successes, &m.FirstTokenMs, &m.BilledInputTokens, &m.BilledCacheTokens, &m.LastSuccessAt)
	if err != nil {
		return m, err
	}
	err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM ops_error_logs WHERE account_id=$1 AND COALESCE(NULLIF(requested_model,''),model)=$2 AND created_at>=$3
 AND COALESCE(upstream_status_code,status_code,0)>=400 AND error_phase='upstream'`, t.AccountID, t.Model, since).Scan(&m.Errors)
	m.Requests = m.Successes + m.Errors
	return m, err
}

func (s *Store) Cleanup(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM quality_monitor_runs WHERE id IN (
 SELECT id FROM (SELECT id,COALESCE(finished_at,created_at) AS checked_at,ROW_NUMBER() OVER(PARTITION BY target_id,kind ORDER BY COALESCE(finished_at,created_at) DESC) AS n
 FROM quality_monitor_runs WHERE status='finished') h WHERE (n>180 AND checked_at<NOW()-INTERVAL '24 hours') OR checked_at<NOW()-INTERVAL '30 days' LIMIT 1000)`)
	return err
}

// Enrich batches overview queries; rendering 1000 targets must not issue 1000
// separate log/history queries or return every generated prompt/answer.
func (s *Store) Enrich(ctx context.Context, targets []Target, since time.Time) error {
	byID := make(map[string]int, len(targets))
	for i := range targets {
		byID[targets[i].ID] = i
	}
	rows, err := s.DB.QueryContext(ctx, `WITH watched AS (
 SELECT id,account_id,model FROM quality_monitor_targets
), successes AS (
 SELECT w.id,COUNT(*) AS n,AVG(u.first_token_ms) AS ttft,COALESCE(SUM(u.input_tokens),0) AS input,
 COALESCE(SUM(u.cache_read_tokens),0) AS cached,MAX(u.created_at) AS last_success
 FROM watched w JOIN usage_logs u ON u.account_id=w.account_id
 AND COALESCE(NULLIF(u.requested_model,''),u.model)=w.model AND u.created_at>=$1 GROUP BY w.id
), failures AS (
 SELECT w.id,COUNT(*) AS n FROM watched w JOIN ops_error_logs e ON e.account_id=w.account_id
 AND COALESCE(NULLIF(e.requested_model,''),e.model)=w.model AND e.created_at>=$1
 AND COALESCE(e.upstream_status_code,e.status_code,0)>=400 AND e.error_phase='upstream' GROUP BY w.id
) SELECT w.id,COALESCE(s.n,0),COALESCE(f.n,0),s.ttft,COALESCE(s.input,0),COALESCE(s.cached,0),s.last_success
 FROM watched w LEFT JOIN successes s ON s.id=w.id LEFT JOIN failures f ON f.id=w.id`, since)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var m Metrics
		if err = rows.Scan(&id, &m.Successes, &m.Errors, &m.FirstTokenMs, &m.BilledInputTokens, &m.BilledCacheTokens, &m.LastSuccessAt); err != nil {
			_ = rows.Close()
			return err
		}
		m.Requests = m.Successes + m.Errors
		if i, ok := byID[id]; ok {
			targets[i].Metrics = m
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	probeRows, err := s.DB.QueryContext(ctx, `SELECT target_id,COUNT(*) FILTER(WHERE verdict='passed'),COUNT(*) FILTER(WHERE verdict='unavailable')
 FROM quality_monitor_runs WHERE kind='availability' AND status='finished' AND finished_at>=$1 GROUP BY target_id`, since)
	if err != nil {
		return err
	}
	for probeRows.Next() {
		var id string
		var successes, failures int
		if err = probeRows.Scan(&id, &successes, &failures); err != nil {
			_ = probeRows.Close()
			return err
		}
		if i, ok := byID[id]; ok {
			targets[i].ProbeSuccesses = successes
			targets[i].ProbeFailures = failures
		}
	}
	err = probeRows.Err()
	_ = probeRows.Close()
	if err != nil {
		return err
	}
	columns := strings.Replace(runColumns, "result,", "(result - 'items' - 'cache_prefix') AS result,", 1)
	rows, err = s.DB.QueryContext(ctx, `SELECT `+columns+` FROM (
 SELECT r.*,ROW_NUMBER() OVER(PARTITION BY target_id,kind,status ORDER BY COALESCE(finished_at,created_at) DESC) AS n
 FROM quality_monitor_runs r WHERE COALESCE(finished_at,created_at)>=NOW()-INTERVAL '30 days'
) h WHERE n<=CASE WHEN kind='availability' AND status='finished' THEN 180 ELSE 1 END ORDER BY COALESCE(finished_at,created_at) DESC`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return err
		}
		i, ok := byID[r.TargetID]
		if !ok {
			continue
		}
		t := &targets[i]
		r.Profile = nil
		if r.Status != "finished" {
			t.Pending = &r
		} else {
			switch r.Kind {
			case "availability":
				t.Recent = append(t.Recent, r)
			case "quality":
				t.LatestQuality = &r
			case "cache":
				t.LatestCache = &r
			}
		}
	}
	return rows.Err()
}
