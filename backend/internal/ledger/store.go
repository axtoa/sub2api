package ledger

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Store struct{ DB *sql.DB }
type scanner interface{ Scan(...any) error }

const entryColumns = `id,kind,amount::text,currency,category,occurred_on::text,counterparty,note,resource_id,voided,version,created_at`
const resourceColumns = `id,name,kind,provider,expires_at,renewal_amount::text,currency,management_url,note,active,version`

func scanEntry(row scanner) (Entry, error) {
	var e Entry
	err := row.Scan(&e.ID, &e.Kind, &e.Amount, &e.Currency, &e.Category, &e.OccurredOn, &e.Counterparty, &e.Note, &e.ResourceID, &e.Voided, &e.Version, &e.CreatedAt)
	return e, err
}
func scanResource(row scanner) (Resource, error) {
	var r Resource
	err := row.Scan(&r.ID, &r.Name, &r.Kind, &r.Provider, &r.ExpiresAt, &r.RenewalAmount, &r.Currency, &r.ManagementURL, &r.Note, &r.Active, &r.Version)
	return r, err
}

// Save the idempotency response in the same transaction as the money and dates.
// A timed-out client can retry without creating a second expense or renewal.
func mutate[T any](ctx context.Context, s *Store, key, action, id string, actor int64, input any, work func(*sql.Tx) (T, error)) (T, error) {
	var zero T
	if _, err := uuid.Parse(key); err != nil {
		return zero, invalid("请提供有效的操作编号")
	}
	raw, err := json.Marshal([]any{action, id, actor, input})
	if err != nil {
		return zero, err
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO ledger_operations(id,request_hash) VALUES($1,$2) ON CONFLICT DO NOTHING`, key, digest)
	if err != nil {
		return zero, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return zero, err
	}
	if n == 0 {
		var previous string
		var response []byte
		if err = tx.QueryRowContext(ctx, `SELECT request_hash,response FROM ledger_operations WHERE id=$1`, key).Scan(&previous, &response); err != nil {
			return zero, err
		}
		if previous != digest {
			return zero, conflict()
		}
		err = json.Unmarshal(response, &zero)
		return zero, err
	}
	result, err := work(tx)
	if err != nil {
		return zero, err
	}
	raw, err = json.Marshal(result)
	if err != nil {
		return zero, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE ledger_operations SET response=$1 WHERE id=$2`, string(raw), key); err != nil {
		return zero, err
	}
	return result, tx.Commit()
}

func audit(ctx context.Context, tx *sql.Tx, id string, actor int64, action string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ledger_events(entity_id,actor_id,action,before_value,after_value) VALUES($1,$2,$3,$4,$5)`, id, actor, action, string(b), string(a))
	return err
}

func insertResource(ctx context.Context, tx *sql.Tx, in ResourceInput, actor int64) (Resource, error) {
	r, err := scanResource(tx.QueryRowContext(ctx, `INSERT INTO ledger_resources(id,name,kind,provider,expires_at,renewal_amount,currency,management_url,note,active)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+resourceColumns, uuid.NewString(), in.Name, in.Kind, in.Provider, in.ExpiresAt, in.RenewalAmount, in.Currency, in.ManagementURL, in.Note, in.Active))
	if err == nil {
		err = audit(ctx, tx, r.ID, actor, "resource_created", nil, r)
	}
	return r, err
}

func insertEntry(ctx context.Context, tx *sql.Tx, in EntryInput, actor int64) (Entry, error) {
	if in.ResourceID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_resources WHERE id=$1)`, *in.ResourceID).Scan(&exists); err != nil {
			return Entry{}, err
		}
		if !exists {
			return Entry{}, invalid("关联资源不存在")
		}
	}
	e, err := scanEntry(tx.QueryRowContext(ctx, `INSERT INTO ledger_entries(id,kind,amount,currency,category,occurred_on,counterparty,note,resource_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+entryColumns, uuid.NewString(), in.Kind, in.Amount, in.Currency, in.Category, in.OccurredOn, in.Counterparty, in.Note, in.ResourceID))
	if err == nil {
		err = audit(ctx, tx, e.ID, actor, "entry_created", nil, e)
	}
	return e, err
}

func (s *Store) SaveEntry(ctx context.Context, id string, in EntryInput, actor int64) (Entry, error) {
	if err := in.validate(); err != nil {
		return Entry{}, err
	}
	if id != "" && in.NewResource != nil {
		return Entry{}, invalid("修改流水时请先建立资源再关联")
	}
	return mutate(ctx, s, in.RequestKey, "entry_save", id, actor, in, func(tx *sql.Tx) (Entry, error) {
		if id == "" {
			if in.NewResource != nil {
				r, err := insertResource(ctx, tx, *in.NewResource, actor)
				if err != nil {
					return Entry{}, err
				}
				in.ResourceID = &r.ID
			}
			return insertEntry(ctx, tx, in, actor)
		}
		before, err := scanEntry(tx.QueryRowContext(ctx, `SELECT `+entryColumns+` FROM ledger_entries WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return Entry{}, err
		}
		if before.Version != in.Version || before.Voided {
			return Entry{}, conflict()
		}
		if in.ResourceID != nil {
			var exists bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_resources WHERE id=$1)`, *in.ResourceID).Scan(&exists)
			if err != nil {
				return Entry{}, err
			}
			if !exists {
				return Entry{}, invalid("关联资源不存在")
			}
		}
		e, err := scanEntry(tx.QueryRowContext(ctx, `UPDATE ledger_entries SET kind=$1,amount=$2,currency=$3,category=$4,occurred_on=$5,counterparty=$6,note=$7,resource_id=$8,version=version+1,updated_at=NOW() WHERE id=$9 RETURNING `+entryColumns, in.Kind, in.Amount, in.Currency, in.Category, in.OccurredOn, in.Counterparty, in.Note, in.ResourceID, id))
		if err == nil {
			err = audit(ctx, tx, id, actor, "entry_updated", before, e)
		}
		return e, err
	})
}

type VersionInput struct {
	Version    int    `json:"version"`
	RequestKey string `json:"request_key"`
}

func (s *Store) VoidEntry(ctx context.Context, id string, in VersionInput, actor int64) (Entry, error) {
	return mutate(ctx, s, in.RequestKey, "entry_void", id, actor, in, func(tx *sql.Tx) (Entry, error) {
		before, err := scanEntry(tx.QueryRowContext(ctx, `SELECT `+entryColumns+` FROM ledger_entries WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return Entry{}, err
		}
		if before.Voided || before.Version != in.Version {
			return Entry{}, conflict()
		}
		e, err := scanEntry(tx.QueryRowContext(ctx, `UPDATE ledger_entries SET voided=TRUE,version=version+1,updated_at=NOW() WHERE id=$1 RETURNING `+entryColumns, id))
		if err == nil {
			err = audit(ctx, tx, id, actor, "entry_voided", before, e)
		}
		return e, err
	})
}

func (s *Store) SaveResource(ctx context.Context, id string, in ResourceInput, actor int64) (Resource, error) {
	if err := in.validate(); err != nil {
		return Resource{}, err
	}
	return mutate(ctx, s, in.RequestKey, "resource_save", id, actor, in, func(tx *sql.Tx) (Resource, error) {
		if id == "" {
			return insertResource(ctx, tx, in, actor)
		}
		before, err := scanResource(tx.QueryRowContext(ctx, `SELECT `+resourceColumns+` FROM ledger_resources WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return Resource{}, err
		}
		if before.Version != in.Version {
			return Resource{}, conflict()
		}
		r, err := scanResource(tx.QueryRowContext(ctx, `UPDATE ledger_resources SET name=$1,kind=$2,provider=$3,expires_at=$4,renewal_amount=$5,currency=$6,management_url=$7,note=$8,active=$9,version=version+1,updated_at=NOW() WHERE id=$10 RETURNING `+resourceColumns, in.Name, in.Kind, in.Provider, in.ExpiresAt, in.RenewalAmount, in.Currency, in.ManagementURL, in.Note, in.Active, id))
		if err == nil {
			err = audit(ctx, tx, id, actor, "resource_updated", before, r)
		}
		return r, err
	})
}

type RenewalResult struct {
	Resource Resource `json:"resource"`
	Entry    *Entry   `json:"entry"`
}

func (s *Store) Renew(ctx context.Context, id string, in RenewalInput, actor int64) (RenewalResult, error) {
	if in.ExpiresAt.IsZero() || in.ExpiresAt.Year() < 1900 || in.ExpiresAt.Year() > 2200 || utf8.RuneCountInString(in.Note) > 3000 {
		return RenewalResult{}, invalid("请填写有效的新到期时间及备注")
	}
	entry := EntryInput{Kind: "expense", Amount: in.Amount, Currency: in.Currency, Category: "续费", OccurredOn: in.OccurredOn, Note: in.Note, ResourceID: &id}
	if in.RecordExpense {
		if err := entry.validate(); err != nil {
			return RenewalResult{}, err
		}
	}
	return mutate(ctx, s, in.RequestKey, "resource_renew", id, actor, in, func(tx *sql.Tx) (RenewalResult, error) {
		before, err := scanResource(tx.QueryRowContext(ctx, `SELECT `+resourceColumns+` FROM ledger_resources WHERE id=$1 FOR UPDATE`, id))
		if err != nil {
			return RenewalResult{}, err
		}
		if before.Version != in.Version {
			return RenewalResult{}, conflict()
		}
		if !in.ExpiresAt.After(before.ExpiresAt) {
			return RenewalResult{}, invalid("续费后的到期时间须晚于原到期时间；修正日期请使用编辑")
		}
		r, err := scanResource(tx.QueryRowContext(ctx, `UPDATE ledger_resources SET expires_at=$1,active=TRUE,version=version+1,updated_at=NOW() WHERE id=$2 RETURNING `+resourceColumns, in.ExpiresAt, id))
		if err != nil {
			return RenewalResult{}, err
		}
		result := RenewalResult{Resource: r}
		if in.RecordExpense {
			entry.Counterparty = before.Provider
			entry.Category = map[string]string{"domain": "域名", "server": "服务器", "other": "资源续费"}[before.Kind]
			e, err := insertEntry(ctx, tx, entry, actor)
			if err != nil {
				return RenewalResult{}, err
			}
			result.Entry = &e
		}
		err = audit(ctx, tx, id, actor, "resource_renewed", before, result)
		return result, err
	})
}

type Filter struct {
	Month, Query, Kind, Currency, ResourceID, Status string
	Page                                             int
}

func (f Filter) validate() error {
	if utf8.RuneCountInString(f.Query) > 200 || f.Page < 1 || f.Page > 100000 || (f.Month != "" && (!validDate(f.Month+"-01") || len(f.Month) != 7)) || (f.Kind != "" && f.Kind != "income" && f.Kind != "expense" && f.Kind != "gift") || (f.Currency != "" && !validCurrency(f.Currency, true)) || (f.Status != "active" && f.Status != "all" && f.Status != "voided") {
		return invalid("筛选条件无效")
	}
	if f.ResourceID != "" {
		if _, err := uuid.Parse(f.ResourceID); err != nil {
			return invalid("资源编号无效")
		}
	}
	return nil
}

const entryWhere = ` WHERE ($1='' OR to_char(occurred_on,'YYYY-MM')=$1) AND ($2='' OR category ILIKE '%'||$2||'%' OR note ILIKE '%'||$2||'%' OR counterparty ILIKE '%'||$2||'%')
 AND ($3='' OR kind=$3) AND ($4='' OR currency=$4) AND ($5='' OR resource_id::text=$5) AND ($6='all' OR ($6='active' AND NOT voided) OR ($6='voided' AND voided))`

func (f Filter) args() []any {
	return []any{f.Month, strings.TrimSpace(f.Query), f.Kind, f.Currency, f.ResourceID, f.Status}
}

type EntryList struct {
	Items      []Entry  `json:"items"`
	Total      int      `json:"total"`
	Totals     []Total  `json:"totals"`
	Categories []string `json:"categories"`
	Today      string   `json:"today"`
	PageSize   int      `json:"page_size"`
}

func (s *Store) Entries(ctx context.Context, f Filter, export bool) (EntryList, error) {
	out := EntryList{Items: []Entry{}, Totals: []Total{}, Categories: []string{}, PageSize: 30}
	if err := f.validate(); err != nil {
		return out, err
	}
	// One snapshot keeps the visible rows, totals and count consistent during edits.
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()
	args := f.args()
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries`+entryWhere, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	limit, offset := out.PageSize, (f.Page-1)*out.PageSize
	if export {
		limit, offset = 10000, 0
		if out.Total > limit {
			return out, invalid("导出最多一万条，请缩小筛选范围")
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+entryColumns+` FROM ledger_entries`+entryWhere+` ORDER BY occurred_on DESC,created_at DESC,id LIMIT $7 OFFSET $8`, append(args, limit, offset)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		e, eErr := scanEntry(rows)
		if eErr != nil {
			_ = rows.Close()
			return out, eErr
		}
		out.Items = append(out.Items, e)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT currency,COALESCE(SUM(amount) FILTER(WHERE kind='income' AND NOT voided),0)::text,COALESCE(SUM(amount) FILTER(WHERE kind='expense' AND NOT voided),0)::text,COALESCE(SUM(amount) FILTER(WHERE kind='gift' AND NOT voided),0)::text,
 COALESCE(SUM(CASE WHEN voided OR kind='gift' THEN 0 WHEN kind='income' THEN amount ELSE -amount END),0)::text FROM ledger_entries`+entryWhere+` GROUP BY currency ORDER BY currency`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var t Total
		if err = rows.Scan(&t.Currency, &t.Income, &t.Expense, &t.Gift, &t.Net); err != nil {
			_ = rows.Close()
			return out, err
		}
		out.Totals = append(out.Totals, t)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT DISTINCT category FROM ledger_entries ORDER BY category LIMIT 500`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c string
		if err = rows.Scan(&c); err != nil {
			_ = rows.Close()
			return out, err
		}
		out.Categories = append(out.Categories, c)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func (s *Store) Resources(ctx context.Context) ([]Resource, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+resourceColumns+` FROM ledger_resources ORDER BY active DESC,expires_at,name,id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Resource{}
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}
func (s *Store) History(ctx context.Context, id string) ([]Event, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT actor_id,action,before_value,after_value,created_at FROM ledger_events WHERE entity_id=$1 ORDER BY id DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Event{}
	for rows.Next() {
		var e Event
		var before, after json.RawMessage
		if err = rows.Scan(&e.ActorID, &e.Action, &before, &after, &e.At); err != nil {
			return nil, err
		}
		e.Before, e.After = before, after
		items = append(items, e)
	}
	return items, rows.Err()
}
func (s *Store) Settings(ctx context.Context) (Settings, error) {
	var c Settings
	var recipients, days []byte
	err := s.DB.QueryRowContext(ctx, `SELECT email_enabled,recipients,reminder_days,version FROM ledger_settings WHERE id=TRUE`).Scan(&c.EmailEnabled, &recipients, &days, &c.Version)
	if err == nil {
		err = json.Unmarshal(recipients, &c.Recipients)
	}
	if err == nil {
		err = json.Unmarshal(days, &c.ReminderDays)
	}
	return c, err
}
func (s *Store) SaveSettings(ctx context.Context, c Settings, actor int64) (Settings, error) {
	if err := c.validate(); err != nil {
		return c, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return c, err
	}
	defer func() { _ = tx.Rollback() }()
	var before Settings
	var oldRecipients, oldDays []byte
	err = tx.QueryRowContext(ctx, `SELECT email_enabled,recipients,reminder_days,version FROM ledger_settings WHERE id=TRUE FOR UPDATE`).Scan(&before.EmailEnabled, &oldRecipients, &oldDays, &before.Version)
	if err != nil {
		return c, err
	}
	if before.Version != c.Version {
		return c, conflict()
	}
	if err = json.Unmarshal(oldRecipients, &before.Recipients); err != nil {
		return c, err
	}
	if err = json.Unmarshal(oldDays, &before.ReminderDays); err != nil {
		return c, err
	}
	r, _ := json.Marshal(c.Recipients)
	d, _ := json.Marshal(c.ReminderDays)
	err = tx.QueryRowContext(ctx, `UPDATE ledger_settings SET email_enabled=$1,recipients=$2,reminder_days=$3,version=version+1 WHERE id=TRUE AND version=$4 RETURNING version`, c.EmailEnabled, string(r), string(d), c.Version).Scan(&c.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return c, conflict()
	}
	if err != nil {
		return c, err
	}
	if err = audit(ctx, tx, "00000000-0000-4000-8000-000000000001", actor, "settings_updated", before, c); err != nil {
		return c, err
	}
	return c, tx.Commit()
}
