package userinsights

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const redeemedBalance = "type = 'balance' AND status = 'used' AND value > 0 AND used_by IS NOT NULL"

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

type Summary struct {
	Amount string `json:"amount"`
	Codes  int64  `json:"codes"`
	Users  int64  `json:"users"`
}

type User struct {
	ID             int64      `json:"id"`
	Email          string     `json:"email"`
	Username       string     `json:"username"`
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
	switch f.Segment {
	case "all", "redeemed", "repeat", "inactive30":
		return nil
	default:
		return fmt.Errorf("invalid segment")
	}
}

var sortColumns = map[string]string{
	"email": "i.email", "redeemed_amount": "i.redeemed_amount",
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

const redemptionCTE = `WITH redemption AS (
    SELECT used_by AS user_id, COUNT(*) AS redeem_count,
           SUM(value) AS redeemed_amount, MIN(used_at) AS first_redeemed_at,
           MAX(used_at) AS last_redeemed_at
    FROM redeem_codes WHERE ` + redeemedBalance + ` GROUP BY used_by
)`

func filteredUsersSQL(segment string) string {
	where := `u.deleted_at IS NULL AND EXISTS (
        SELECT 1 FROM usage_logs used WHERE used.user_id = u.id
    ) AND
        (POSITION(LOWER($1) IN LOWER(u.email)) > 0 OR
         POSITION(LOWER($1) IN LOWER(u.username)) > 0 OR u.id::text = $1)`
	switch segment {
	case "redeemed":
		where += " AND r.redeem_count > 0"
	case "repeat":
		where += " AND r.redeem_count >= 2"
	case "inactive30":
		where += ` AND r.redeem_count > 0 AND NOT EXISTS (
            SELECT 1 FROM usage_logs ul WHERE ul.user_id = u.id
            AND ul.created_at >= $2
        )`
	}
	return ` FROM users u LEFT JOIN redemption r ON r.user_id = u.id WHERE ` + where
}

func (s *Store) List(ctx context.Context, f Filter) (*ListResult, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	since := time.Now().UTC().AddDate(0, 0, -30)
	result := &ListResult{Items: []User{}, Page: f.Page, PageSize: f.Size}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value), 0)::text,
        COUNT(*), COUNT(DISTINCT used_by) FROM redeem_codes WHERE `+redeemedBalance).
		Scan(&result.Summary.Amount, &result.Summary.Codes, &result.Summary.Users); err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	args := []any{strings.TrimSpace(f.Search)}
	if f.Segment == "inactive30" {
		args = append(args, since)
	}
	base := redemptionCTE + ` SELECT COUNT(*), COALESCE(SUM(r.redeemed_amount), 0)::text` + filteredUsersSQL(f.Segment)
	if err := s.db.QueryRowContext(ctx, base, args...).Scan(&result.Total, &result.FilteredAmount); err != nil {
		return nil, fmt.Errorf("filtered summary: %w", err)
	}
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	sinceArg := len(args) + 3
	query := redemptionCTE + `, candidates AS (
        SELECT u.id, u.email, u.username, u.status, u.balance,
               COALESCE(r.redeemed_amount, 0) AS redeemed_amount,
               COALESCE(r.redeem_count, 0) AS redeem_count, r.last_redeemed_at
        ` + filteredUsersSQL(f.Segment) + `
    ), usage_stats AS (
        SELECT ul.user_id,
               COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $` + fmt.Sprint(sinceArg) + `), 0) AS usage_30d,
               COUNT(*) FILTER (WHERE ul.created_at >= $` + fmt.Sprint(sinceArg) + `) AS calls_30d,
               COUNT(*) AS total_calls, MAX(ul.created_at) AS last_used_at
        FROM usage_logs ul JOIN candidates p ON p.id = ul.user_id
        GROUP BY ul.user_id
    ), insights AS (
        SELECT p.*, usage_stats.usage_30d, usage_stats.calls_30d,
               usage_stats.total_calls, usage_stats.last_used_at
        FROM candidates p JOIN usage_stats ON usage_stats.user_id = p.id
    )
    SELECT i.id, i.email, i.username, i.status, i.balance::text,
           i.redeemed_amount::text, i.redeem_count, i.last_redeemed_at,
           i.usage_30d::text, i.calls_30d, i.total_calls, i.last_used_at
    FROM insights i ORDER BY ` + f.orderBySQL() + `
    LIMIT $` + fmt.Sprint(limitArg) + ` OFFSET $` + fmt.Sprint(offsetArg)
	args = append(args, f.Size, (f.Page-1)*f.Size, since)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item User
		var redeemedAt, usedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Email, &item.Username, &item.Status, &item.Balance,
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

func (s *Store) Detail(ctx context.Context, userID int64) (*Detail, error) {
	if userID <= 0 {
		return nil, sql.ErrNoRows
	}
	result := &Detail{Redemptions: []Redemption{}, Models: []Model{}}
	var firstAt, lastAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.email, u.username, u.status, u.balance::text,
               COALESCE(r.redeemed_amount, 0)::text, COALESCE(r.redeem_count, 0),
               r.first_redeemed_at, r.last_redeemed_at
        FROM users u LEFT JOIN LATERAL (
            SELECT COUNT(*) AS redeem_count, SUM(value) AS redeemed_amount,
                   MIN(used_at) AS first_redeemed_at, MAX(used_at) AS last_redeemed_at
            FROM redeem_codes WHERE used_by = u.id AND `+redeemedBalance+`
        ) r ON true
        WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).
		Scan(&result.ID, &result.Email, &result.Username, &result.Status, &result.Balance,
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
			rows.Close()
			return nil, fmt.Errorf("scan redemption: %w", err)
		}
		result.Redemptions = append(result.Redemptions, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

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
	defer rows.Close()
	for rows.Next() {
		var item Model
		if err := rows.Scan(&item.Name, &item.Calls, &item.Cost); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		result.Models = append(result.Models, item)
	}
	return result, rows.Err()
}
