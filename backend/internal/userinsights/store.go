package userinsights

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const redeemedBalance = "type = 'balance' AND status = 'used' AND value > 0 AND used_by IS NOT NULL"

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

type Summary struct {
	Amount      string `json:"amount"`
	Codes       int64  `json:"codes"`
	Users       int64  `json:"users"`
	TodayAmount string `json:"today_amount"`
}

type User struct {
	ID             int64      `json:"id"`
	Email          string     `json:"email"`
	Username       string     `json:"username"`
	Notes          string     `json:"notes"`
	Status         string     `json:"status"`
	Balance        string     `json:"balance"`
	RedeemedAmount string     `json:"redeemed_amount"`
	RedeemCount    int64      `json:"redeem_count"`
	LastRedeemedAt *time.Time `json:"last_redeemed_at"`
	Usage30d       string     `json:"usage_30d"`
	Calls30d       int64      `json:"calls_30d"`
	TotalCalls     int64      `json:"total_calls"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}

type ListResult struct {
	Items          []User  `json:"items"`
	Total          int64   `json:"total"`
	Page           int     `json:"page"`
	PageSize       int     `json:"page_size"`
	Summary        Summary `json:"summary"`
	FilteredAmount string  `json:"filtered_amount"`
	IgnoredCount   int     `json:"ignored_count"`
}

type IgnoreUser struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type CandidateFilter struct {
	Search string
	Limit  int
}

type Redemption struct {
	Value  string    `json:"value"`
	UsedAt time.Time `json:"used_at"`
}

type Model struct {
	Name  string `json:"name"`
	Calls int64  `json:"calls"`
	Cost  string `json:"cost"`
}

type Detail struct {
	User
	FirstRedeemedAt  *time.Time   `json:"first_redeemed_at"`
	AdminAddedAmount string       `json:"admin_added_amount"`
	Redemptions      []Redemption `json:"redemptions"`
	Models           []Model      `json:"models"`
}

type Filter struct {
	Search    string
	Segment   string
	Page      int
	Size      int
	SortBy    string
	SortOrder string
	From      time.Time
	To        time.Time
	TodayFrom time.Time
	TodayTo   time.Time
}

func (f Filter) Validate() error {
	if f.Page < 1 || f.Page > 100000 || f.Size < 1 || f.Size > 50 || len([]rune(f.Search)) > 100 {
		return fmt.Errorf("invalid pagination or search")
	}
	if f.SortBy != "" {
		if _, ok := sortColumns[f.SortBy]; !ok {
			return fmt.Errorf("invalid sort column")
		}
	}
	if f.SortOrder != "" && f.SortOrder != "asc" && f.SortOrder != "desc" {
		return fmt.Errorf("invalid sort order")
	}
	if (!f.From.IsZero() || !f.To.IsZero()) && (f.From.IsZero() || f.To.IsZero() || !f.From.Before(f.To) || f.To.Sub(f.From) > 366*24*time.Hour) {
		return fmt.Errorf("invalid date range")
	}
	if (!f.TodayFrom.IsZero() || !f.TodayTo.IsZero()) && (f.TodayFrom.IsZero() || f.TodayTo.IsZero() || !f.TodayFrom.Before(f.TodayTo)) {
		return fmt.Errorf("invalid today range")
	}
	switch f.Segment {
	case "all", "redeemed", "repeat", "inactive30":
		return nil
	default:
		return fmt.Errorf("invalid segment")
	}
}

var sortColumns = map[string]string{
	"email": "i.email", "notes": "i.notes", "redeemed_amount": "i.redeemed_amount",
	"redeem_count": "i.redeem_count", "last_redeemed_at": "i.last_redeemed_at",
	"balance": "i.balance", "usage_30d": "i.usage_30d",
	"calls_30d": "i.calls_30d", "total_calls": "i.total_calls", "last_used_at": "i.last_used_at",
}

func (f Filter) orderBySQL() string {
	column := sortColumns[f.SortBy]
	if column == "" {
		column = sortColumns["redeemed_amount"]
	}
	direction := "DESC"
	if f.SortOrder == "asc" {
		direction = "ASC"
	}
	return column + " " + direction + " NULLS LAST, i.id DESC"
}

const ignoredUsers = `u.id NOT IN (
    SELECT value::bigint FROM jsonb_array_elements_text(
        COALESCE((SELECT value::jsonb FROM settings WHERE key = 'user_insights_ignored_ids'), '[]'::jsonb)
    )
)`

func redemptionCTE(fromArg, toArg int) string {
	return `WITH redemption AS (
    SELECT used_by AS user_id, COUNT(*) AS redeem_count,
           SUM(value) AS redeemed_amount, MIN(used_at) AS first_redeemed_at,
           MAX(used_at) AS last_redeemed_at
    FROM redeem_codes WHERE ` + redeemedBalance + ` AND used_at >= $` + fmt.Sprint(fromArg) + ` AND used_at < $` + fmt.Sprint(toArg) + ` GROUP BY used_by
)`
}

func filteredUsersSQL(segment string, inactiveSinceArg int) string {
	where := `u.deleted_at IS NULL AND ` + ignoredUsers + ` AND EXISTS (
        SELECT 1 FROM usage_logs used WHERE used.user_id = u.id
    ) AND
		 (POSITION(LOWER($1) IN LOWER(u.email)) > 0 OR
         POSITION(LOWER($1) IN LOWER(u.username)) > 0 OR u.id::text = $1) AND r.redeem_count > 0`
	switch segment {
	case "redeemed":
		where += " AND r.redeem_count > 0"
	case "repeat":
		where += " AND r.redeem_count >= 2"
	case "inactive30":
		where += ` AND NOT EXISTS (
            SELECT 1 FROM usage_logs ul WHERE ul.user_id = u.id
            AND ul.created_at >= $` + fmt.Sprint(inactiveSinceArg) + `
        )`
	}
	return ` FROM users u LEFT JOIN redemption r ON r.user_id = u.id WHERE ` + where
}

func (s *Store) List(ctx context.Context, f Filter) (*ListResult, error) {
	now := time.Now()
	if f.TodayFrom.IsZero() {
		f.TodayFrom = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	if f.TodayTo.IsZero() {
		f.TodayTo = now
	}
	if f.From.IsZero() {
		f.From = f.TodayFrom
	}
	if f.To.IsZero() {
		f.To = f.TodayTo
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	result := &ListResult{Items: []User{}, Page: f.Page, PageSize: f.Size}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(rc.value), 0)::text,
	    COUNT(*), COUNT(DISTINCT rc.used_by), COALESCE(SUM(rc.value) FILTER (WHERE rc.used_at >= $1 AND rc.used_at < $2), 0)::text
	    , jsonb_array_length(COALESCE((SELECT value::jsonb FROM settings WHERE key = 'user_insights_ignored_ids'), '[]'::jsonb))
	    FROM redeem_codes rc WHERE rc.type = 'balance' AND rc.status = 'used' AND rc.value > 0 AND rc.used_by IS NOT NULL
	    AND rc.used_by NOT IN (SELECT value::bigint FROM jsonb_array_elements_text(COALESCE((SELECT value::jsonb FROM settings WHERE key = 'user_insights_ignored_ids'), '[]'::jsonb)))`, f.TodayFrom, f.TodayTo).
		Scan(&result.Summary.Amount, &result.Summary.Codes, &result.Summary.Users, &result.Summary.TodayAmount, &result.IgnoredCount); err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	search := strings.TrimSpace(f.Search)
	baseArgs := []any{search, f.From, f.To}
	baseFromArg, baseToArg := 2, 3
	baseInactiveArg := 0
	if f.Segment == "inactive30" {
		baseArgs = append(baseArgs, time.Now().UTC().AddDate(0, 0, -30))
		baseInactiveArg = 4
	}
	base := redemptionCTE(baseFromArg, baseToArg) + ` SELECT COUNT(*), COALESCE(SUM(r.redeemed_amount), 0)::text` + filteredUsersSQL(f.Segment, baseInactiveArg)
	if err := s.db.QueryRowContext(ctx, base, baseArgs...).Scan(&result.Total, &result.FilteredAmount); err != nil {
		return nil, fmt.Errorf("filtered summary: %w", err)
	}
	args := []any{search, f.From, f.To}
	fromArg, toArg := 2, 3
	inactiveSinceArg := 0
	if f.Segment == "inactive30" {
		args = append(args, time.Now().UTC().AddDate(0, 0, -30))
		inactiveSinceArg = len(args)
	}
	usageSinceArg := len(args) + 1
	args = append(args, time.Now().UTC().AddDate(0, 0, -30))
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	query := redemptionCTE(fromArg, toArg) + `, candidates AS (
        SELECT u.id, u.email, u.username, u.notes, u.status, u.balance,
               COALESCE(r.redeemed_amount, 0) AS redeemed_amount,
               COALESCE(r.redeem_count, 0) AS redeem_count, r.last_redeemed_at
        ` + filteredUsersSQL(f.Segment, inactiveSinceArg) + `
    ), usage_stats AS (
        SELECT ul.user_id,
               COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $` + fmt.Sprint(usageSinceArg) + `), 0) AS usage_30d,
               COUNT(*) FILTER (WHERE ul.created_at >= $` + fmt.Sprint(usageSinceArg) + `) AS calls_30d,
               COUNT(*) AS total_calls, MAX(ul.created_at) AS last_used_at
        FROM usage_logs ul JOIN candidates p ON p.id = ul.user_id
        GROUP BY ul.user_id
    ), insights AS (
        SELECT p.*, usage_stats.usage_30d, usage_stats.calls_30d,
               usage_stats.total_calls, usage_stats.last_used_at
        FROM candidates p JOIN usage_stats ON usage_stats.user_id = p.id
    )
    SELECT i.id, i.email, i.username, i.notes, i.status, i.balance::text,
           i.redeemed_amount::text, i.redeem_count, i.last_redeemed_at,
           i.usage_30d::text, i.calls_30d, i.total_calls, i.last_used_at
    FROM insights i ORDER BY ` + f.orderBySQL() + `
    LIMIT $` + fmt.Sprint(limitArg) + ` OFFSET $` + fmt.Sprint(offsetArg)
	args = append(args, f.Size, (f.Page-1)*f.Size)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item User
		var redeemedAt, usedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Email, &item.Username, &item.Notes, &item.Status, &item.Balance,
			&item.RedeemedAmount, &item.RedeemCount, &redeemedAt, &item.Usage30d,
			&item.Calls30d, &item.TotalCalls, &usedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if redeemedAt.Valid {
			item.LastRedeemedAt = &redeemedAt.Time
		}
		if usedAt.Valid {
			item.LastUsedAt = &usedAt.Time
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

func (s *Store) Ignored(ctx context.Context) ([]IgnoreUser, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.email, u.username FROM users u
		WHERE u.id IN (SELECT value::bigint FROM jsonb_array_elements_text(COALESCE((SELECT value::jsonb FROM settings WHERE key = 'user_insights_ignored_ids'), '[]'::jsonb)))
		ORDER BY u.email, u.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]IgnoreUser, 0)
	for rows.Next() {
		var item IgnoreUser
		if err := rows.Scan(&item.ID, &item.Email, &item.Username); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Candidates(ctx context.Context, f CandidateFilter) ([]IgnoreUser, error) {
	search := strings.TrimSpace(f.Search)
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.email, u.username FROM users u
		WHERE u.deleted_at IS NULL AND EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = u.id)
		AND (POSITION(LOWER($1) IN LOWER(u.email)) > 0 OR POSITION(LOWER($1) IN LOWER(u.username)) > 0 OR u.id::text = $1)
		ORDER BY u.email, u.id LIMIT $2`, search, f.Limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]IgnoreUser, 0)
	for rows.Next() {
		var item IgnoreUser
		if err := rows.Scan(&item.ID, &item.Email, &item.Username); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveIgnored(ctx context.Context, ids []int64) error {
	if len(ids) > 5000 {
		return fmt.Errorf("too many ignored users")
	}
	unique := make(map[int64]struct{}, len(ids))
	clean := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("invalid user id")
		}
		if _, ok := unique[id]; !ok {
			unique[id] = struct{}{}
			clean = append(clean, id)
		}
	}
	sort.Slice(clean, func(i, j int) bool { return clean[i] < clean[j] })
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id IN (SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb))`, string(encodedJSON(clean))).Scan(&count); err != nil {
		return err
	}
	if count != len(clean) {
		return fmt.Errorf("unknown user id")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ('user_insights_ignored_ids', $1, NOW()) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, string(encodedJSON(clean)))
	return err
}

func encodedJSON(ids []int64) []byte { data, _ := json.Marshal(ids); return data }

func (s *Store) Detail(ctx context.Context, userID int64) (*Detail, error) {
	if userID <= 0 {
		return nil, sql.ErrNoRows
	}
	result := &Detail{Redemptions: []Redemption{}, Models: []Model{}}
	var firstAt, lastAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.email, u.username, u.notes, u.status, u.balance::text,
               COALESCE(r.redeemed_amount, 0)::text, COALESCE(r.redeem_count, 0),
               r.first_redeemed_at, r.last_redeemed_at
        FROM users u LEFT JOIN LATERAL (
            SELECT COUNT(*) AS redeem_count, SUM(value) AS redeemed_amount,
                   MIN(used_at) AS first_redeemed_at, MAX(used_at) AS last_redeemed_at
            FROM redeem_codes WHERE used_by = u.id AND `+redeemedBalance+`
        ) r ON true
        WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).
		Scan(&result.ID, &result.Email, &result.Username, &result.Notes, &result.Status, &result.Balance,
			&result.RedeemedAmount, &result.RedeemCount, &firstAt, &lastAt)
	if err != nil {
		return nil, fmt.Errorf("user detail: %w", err)
	}
	if firstAt.Valid {
		result.FirstRedeemedAt = &firstAt.Time
	}
	if lastAt.Valid {
		result.LastRedeemedAt = &lastAt.Time
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value), 0)::text FROM redeem_codes
        WHERE used_by = $1 AND type = 'admin_balance' AND status = 'used' AND value > 0`, userID).
		Scan(&result.AdminAddedAmount); err != nil {
		return nil, fmt.Errorf("admin added amount: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT value::text, used_at FROM redeem_codes
        WHERE used_by = $1 AND `+redeemedBalance+` AND used_at IS NOT NULL
        ORDER BY used_at DESC LIMIT 20`, userID)
	if err != nil {
		return nil, fmt.Errorf("redemptions: %w", err)
	}
	for rows.Next() {
		var item Redemption
		if err := rows.Scan(&item.Value, &item.UsedAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan redemption: %w", err)
		}
		result.Redemptions = append(result.Redemptions, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close redemptions: %w", err)
	}

	since := time.Now().UTC().AddDate(0, 0, -30)
	var usedAt sql.NullTime
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $2), 0)::text,
        COUNT(*) FILTER (WHERE created_at >= $2), COUNT(*), MAX(created_at) FROM usage_logs
        WHERE user_id = $1`, userID, since).
		Scan(&result.Usage30d, &result.Calls30d, &result.TotalCalls, &usedAt)
	if err != nil {
		return nil, fmt.Errorf("usage: %w", err)
	}
	if usedAt.Valid {
		result.LastUsedAt = &usedAt.Time
	}
	rows, err = s.db.QueryContext(ctx, `SELECT COALESCE(NULLIF(requested_model, ''), model) AS name,
        COUNT(*) AS calls, COALESCE(SUM(actual_cost), 0)::text AS cost
        FROM usage_logs WHERE user_id = $1 AND created_at >= $2 AND actual_cost > 0
        GROUP BY 1 ORDER BY calls DESC, SUM(actual_cost) DESC LIMIT 5`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("models: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item Model
		if err := rows.Scan(&item.Name, &item.Calls, &item.Cost); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		result.Models = append(result.Models, item)
	}
	return result, rows.Err()
}
