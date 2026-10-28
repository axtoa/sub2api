package quality

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type Group struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type MonitoredAccount struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	Status      string  `json:"status"`
	Enabled     bool    `json:"enabled"`
	Version     int     `json:"version"`
	Groups      []Group `json:"groups"`
	Unsupported bool    `json:"unsupported"`
}

// Discovery uses local configuration only. It never authenticates to an upstream.
func monitorModels(a *service.Account) []string {
	if a == nil {
		return nil
	}
	mapping := a.GetModelMapping()
	models := []string{}
	if len(mapping) > 0 {
		for model := range mapping {
			if validateModel(model) == nil && service.QualityProbeSupported(a, model) {
				models = append(models, model)
			}
		}
	} else {
		model := openai.DefaultTestModel
		if a.Platform == service.PlatformAnthropic {
			model = claude.DefaultTestModel
		}
		if service.QualityProbeSupported(a, model) {
			models = append(models, model)
		}
	}
	sort.Strings(models)
	return models
}

func (s *Store) SyncAccounts(ctx context.Context, initial bool) (int, error) {
	if _, err := s.Settings(ctx); err != nil {
		return 0, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var initialized bool
	if err = tx.QueryRowContext(ctx, `SELECT inventory_initialized FROM quality_monitor_settings WHERE id=TRUE FOR UPDATE`).Scan(&initialized); err != nil {
		return 0, err
	}
	if initial && initialized {
		return 0, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,platform,type,credentials,extra,parent_account_id,COALESCE((SELECT COALESCE(NULLIF(u.requested_model,''),u.model) FROM usage_logs u WHERE u.account_id=accounts.id ORDER BY u.created_at DESC,u.id DESC LIMIT 1),'') FROM accounts WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return 0, err
	}
	accounts := []*service.Account{}
	recentModels := map[int64]string{}
	for rows.Next() {
		a := &service.Account{}
		var credentials, extra []byte
		var recent string
		if err = rows.Scan(&a.ID, &a.Platform, &a.Type, &credentials, &extra, &a.ParentAccountID, &recent); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if err = json.Unmarshal(credentials, &a.Credentials); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if len(extra) > 0 {
			if err = json.Unmarshal(extra, &a.Extra); err != nil {
				_ = rows.Close()
				return 0, err
			}
		}
		accounts = append(accounts, a)
		recentModels[a.ID] = recent
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	created := 0
	for _, a := range accounts {
		res, e := tx.ExecContext(ctx, `INSERT INTO quality_monitor_accounts(account_id,enabled)
 SELECT $1,COALESCE(BOOL_OR(enabled),TRUE) FROM quality_monitor_targets WHERE account_id=$1 ON CONFLICT DO NOTHING`, a.ID)
		if e != nil {
			return 0, e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return 0, e
		}
		created += int(n)
		models := monitorModels(a)
		recent := recentModels[a.ID]
		if validateModel(recent) == nil && service.QualityProbeSupported(a, recent) {
			models = append([]string{recent}, models...)
		}
		// New accounts get one representative model; existing choices and opt-outs
		// are retained. Further models are explicitly added from account details.
		if len(models) > 1 {
			models = models[:1]
		}
		for _, model := range models {
			_, e = tx.ExecContext(ctx, `INSERT INTO quality_monitor_targets(id,account_id,model) SELECT $1,$2,$3 WHERE NOT EXISTS(SELECT 1 FROM quality_monitor_targets WHERE account_id=$2) ON CONFLICT(account_id,model) DO NOTHING`, uuid.NewString(), a.ID, model)
			if e != nil {
				return 0, e
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE quality_monitor_settings SET inventory_initialized=TRUE WHERE id=TRUE`); err != nil {
		return 0, err
	}
	return created, tx.Commit()
}

func (s *Store) Inventory(ctx context.Context) ([]MonitoredAccount, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id,a.name,a.platform,a.status,m.enabled,m.version,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',g.id,'name',g.name) ORDER BY g.name,g.id)
 FROM account_groups ag JOIN groups g ON g.id=ag.group_id AND g.deleted_at IS NULL WHERE ag.account_id=a.id),'[]'::jsonb),
 (NOT EXISTS(SELECT 1 FROM quality_monitor_targets t WHERE t.account_id=a.id)
 OR a.platform NOT IN ('openai','anthropic') OR a.parent_account_id IS NOT NULL
 OR NOT(a.type IN ('apikey','oauth') OR (a.platform='anthropic' AND a.type='setup-token'))
 OR a.extra @> '{"synthetic_ui_test":true}'::jsonb
 OR (a.platform='openai' AND a.type='oauth' AND LOWER(BTRIM(COALESCE(a.credentials->>'auth_mode','')))='agentidentity'))
 FROM quality_monitor_accounts m JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL ORDER BY a.name,a.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []MonitoredAccount{}
	for rows.Next() {
		var a MonitoredAccount
		var groups []byte
		if err = rows.Scan(&a.ID, &a.Name, &a.Platform, &a.Status, &a.Enabled, &a.Version, &groups, &a.Unsupported); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(groups, &a.Groups); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (s *Store) SaveAccount(ctx context.Context, id int64, enabled bool, version int) error {
	var saved int
	err := s.DB.QueryRowContext(ctx, `UPDATE quality_monitor_accounts SET enabled=$1,version=version+1 WHERE account_id=$2 AND version=$3 RETURNING version`, enabled, id, version).Scan(&saved)
	if err == sql.ErrNoRows {
		return conflict("账号监控设置已修改，请刷新后重试")
	}
	return err
}
