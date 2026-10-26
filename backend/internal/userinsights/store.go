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
	Search  string
	Segment string
	Page    int
	Size    int
}

func (f Filter) Validate() error {
	if f.Page < 1 || f.Page > 100000 || f.Size < 1 || f.Size > 50 || len([]rune(f.Search)) > 100 {
		return fmt.Errorf("invalid pagination or search")
	}
	switch f.Segment {
	case "all", "redeemed", "repeat", "inactive30":
		return nil
	default:
		return fmt.Errorf("invalid segment")
	}
}

const redemptionCTE = `WITH redemption AS (
    SELECT used_by AS user_id, COUNT(*) AS redeem_count,
           SUM(value) AS redeemed_amount, MIN(used_at) AS first_redeemed_at,
           MAX(used_at) AS last_redeemed_at
    FROM redeem_codes WHERE ` + redeemedBalance + ` GROUP BY used_by
)`

func filteredUsersSQL(segment string) string {
	where := `u.deleted_at IS NULL AND
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
            AND ul.created_at >= $2 AND ul.actual_cost > 0
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
	query := redemptionCTE + `, page AS (
        SELECT u.id, u.email, u.username, u.status, u.balance,
               COALESCE(r.redeemed_amount, 0) AS redeemed_amount,
               COALESCE(r.redeem_count, 0) AS redeem_count, r.last_redeemed_at
        ` + filteredUsersSQL(f.Segment) + `
        ORDER BY (r.redeem_count IS NOT NULL) DESC, r.last_redeemed_at DESC NULLS LAST, u.id DESC
        LIMIT $` + fmt.Sprint(limitArg) + ` OFFSET $` + fmt.Sprint(offsetArg) + `
    ), usage30 AS (
        SELECT ul.user_id, COALESCE(SUM(ul.actual_cost), 0)::text AS cost,
               COUNT(*) AS calls, MAX(ul.created_at) AS last_used_at
        FROM usage_logs ul JOIN page p ON p.id = ul.user_id
        WHERE ul.created_at >= $` + fmt.Sprint(sinceArg) + ` AND ul.actual_cost > 0
        GROUP BY ul.user_id
    )
    SELECT p.id, p.email, p.username, p.status, p.balance::text,
           p.redeemed_amount::text, p.redeem_count, p.last_redeemed_at,
           COALESCE(usage30.cost, '0'), COALESCE(usage30.calls, 0), usage30.last_used_at
    FROM page p LEFT JOIN usage30 ON usage30.user_id = p.id
    ORDER BY (p.redeem_count > 0) DESC, p.last_redeemed_at DESC NULLS LAST, p.id DESC`
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
			&item.Calls30d, &usedAt); err != nil {
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
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(actual_cost), 0)::text,
        COUNT(*), MAX(created_at) FROM usage_logs
        WHERE user_id = $1 AND created_at >= $2 AND actual_cost > 0`, userID, since).
		Scan(&result.Usage30d, &result.Calls30d, &lastAt)
	if err != nil {
		return nil, fmt.Errorf("usage: %w", err)
	}
	if lastAt.Valid {
		result.LastUsedAt = &lastAt.Time
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
