// Package upstreambalance observes upstream wallets, independently of gateway billing.
package upstreambalance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Config struct {
	Name            string  `json:"name"`
	Provider        string  `json:"provider"`
	Mode            string  `json:"mode"`
	QueryAccountID  *int64  `json:"query_account_id"`
	AccountIDs      []int64 `json:"account_ids"`
	Currency        string  `json:"currency"`
	AutoEnabled     bool    `json:"auto_enabled"`
	IntervalMinutes int     `json:"interval_minutes"`
	EstimateEnabled bool    `json:"estimate_enabled"`
	LowThreshold    *string `json:"low_threshold"`
	ManagementURL   string  `json:"management_url"`
	Note            string  `json:"note"`
}
type Input struct {
	Config
	Version        int     `json:"version"`
	InitialBalance *string `json:"initial_balance"`
	RequestKey     string  `json:"request_key"`
}
type Calibration struct {
	Amount     string `json:"amount"`
	Version    int    `json:"version"`
	RequestKey string `json:"request_key"`
}
type Observation struct {
	Amount   string    `json:"amount"`
	Currency string    `json:"currency"`
	Scope    string    `json:"scope"` // wallet, key_quota, subscription, manual
	At       time.Time `json:"at"`
}
type Wallet struct {
	ID string `json:"id"`
	Config
	Version           int             `json:"version"`
	Observation       *Observation    `json:"observation"`
	Status            string          `json:"status"`
	LastError         string          `json:"last_error"`
	LastAttemptAt     *time.Time      `json:"last_attempt_at"`
	NextQueryAt       time.Time       `json:"next_query_at"`
	Identity          string          `json:"-"`
	Failures          int             `json:"-"`
	Accounts          []AccountOption `json:"accounts"`
	TodayCost         string          `json:"today_cost"`
	WeekCost          string          `json:"week_cost"`
	MonthCost         string          `json:"month_cost"`
	SinceBaselineCost string          `json:"since_baseline_cost"`
	EstimatedBalance  *string         `json:"estimated_balance"`
	DaysRemaining     *string         `json:"days_remaining"`
	Stale             bool            `json:"stale"`
	Low               bool            `json:"low"`
	UsageIncomplete   bool            `json:"usage_incomplete"`
}
type AccountOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	WalletID  string `json:"wallet_id"`
	TodayCost string `json:"today_cost,omitempty"`
	WeekCost  string `json:"week_cost,omitempty"`
	MonthCost string `json:"month_cost,omitempty"`
}
type Settings struct {
	EmailEnabled bool     `json:"email_enabled"`
	Recipients   []string `json:"recipients"`
	Version      int      `json:"version"`
}
type Delivery struct {
	Day       string `json:"day"`
	Recipient string `json:"recipient"`
	Status    string `json:"status"`
}
type Event struct {
	ActorID int64           `json:"actor_id"`
	Action  string          `json:"action"`
	Detail  json.RawMessage `json:"detail"`
	At      time.Time       `json:"at"`
}

func invalid(s string) error  { return apperrors.BadRequest("UPSTREAM_BALANCE_INVALID", s) }
func conflict(s string) error { return apperrors.Conflict("UPSTREAM_BALANCE_CONFLICT", s) }

var amountPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?$`)

func amount(s string, negative bool) (string, error) {
	if !amountPattern.MatchString(s) || (!negative && strings.HasPrefix(s, "-")) {
		return "", invalid("金额最多九位整数、六位小数")
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", invalid("金额无效")
	}
	return r.FloatString(6), nil
}
func currency(s string) bool {
	switch s {
	case "USD", "CNY", "EUR", "HKD", "GBP", "JPY", "CREDIT":
		return true
	}
	return false
}
func (c *Config) validate() error {
	c.Name, c.Provider, c.Note = strings.TrimSpace(c.Name), strings.TrimSpace(c.Provider), strings.TrimSpace(c.Note)
	if c.Name == "" || utf8.RuneCountInString(c.Name) > 160 || utf8.RuneCountInString(c.Provider) > 160 || utf8.RuneCountInString(c.Note) > 2000 || !currency(c.Currency) || (c.Mode != "sub2api" && c.Mode != "manual") || c.IntervalMinutes < 5 || c.IntervalMinutes > 1440 || len(c.AccountIDs) > 200 {
		return invalid("请核对钱包名称、模式、币种及查询周期（5–1440 分钟）")
	}
	ids := map[int64]bool{}
	for _, id := range c.AccountIDs {
		if id <= 0 || ids[id] {
			return invalid("关联账号无效或重复")
		}
		ids[id] = true
	}
	sort.Slice(c.AccountIDs, func(i, j int) bool { return c.AccountIDs[i] < c.AccountIDs[j] })
	if c.AccountIDs == nil {
		c.AccountIDs = []int64{}
	}
	if c.Mode == "sub2api" {
		if c.QueryAccountID == nil || !ids[*c.QueryAccountID] {
			return invalid("请从关联账号中选择查询来源")
		}
		c.EstimateEnabled = false
	} else {
		c.QueryAccountID = nil
		c.AutoEnabled = false
	}
	if c.EstimateEnabled && (c.Currency != "USD" || len(c.AccountIDs) == 0) {
		return invalid("消耗估算需要关联账号且使用 USD；其他币种不自动折算")
	}
	if c.LowThreshold != nil {
		v, err := amount(*c.LowThreshold, false)
		if err != nil {
			return err
		}
		c.LowThreshold = &v
	}
	c.ManagementURL = strings.TrimSpace(c.ManagementURL)
	if c.ManagementURL != "" {
		u, err := url.Parse(c.ManagementURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(c.ManagementURL) > 2000 || strings.ContainsAny(c.ManagementURL, "\r\n") {
			return invalid("管理入口必须是无登录凭据的 HTTPS 链接")
		}
	}
	return nil
}
func (s *Settings) validate() error {
	if s.Recipients == nil {
		s.Recipients = []string{}
	}
	if len(s.Recipients) > 5 {
		return invalid("最多五个收件邮箱")
	}
	seen := map[string]bool{}
	for i, v := range s.Recipients {
		v = strings.ToLower(strings.TrimSpace(v))
		a, e := mail.ParseAddress(v)
		if e != nil || a.Address != v || len(v) > 254 || seen[v] {
			return invalid("邮箱无效或重复")
		}
		seen[v] = true
		s.Recipients[i] = v
	}
	if s.EmailEnabled && len(s.Recipients) == 0 {
		return invalid("请先填写收件邮箱")
	}
	return nil
}
func sourceIdentity(a *service.Account) string {
	if a == nil {
		return ""
	}
	proxy := ""
	if a.Proxy != nil {
		proxy = a.Proxy.URL()
	}
	v, _ := json.Marshal([]any{a.ID, a.Platform, a.Type, a.GetCredential("base_url"), a.GetCredential("api_key"), a.GetCredential("base_url_chat_completions"), a.ProxyID, proxy})
	hash := sha256.Sum256(v)
	return hex.EncodeToString(hash[:])
}
func numeric(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return new(big.Rat)
	}
	return r
}
func (w *Wallet) decorate(now time.Time) {
	w.Stale = w.Observation != nil && w.Mode == "sub2api" && (w.Status != "ok" || now.Sub(w.Observation.At) > time.Duration(w.IntervalMinutes*2)*time.Minute)
	if w.Observation == nil {
		return
	}
	current := numeric(w.Observation.Amount)
	if w.Mode == "manual" && w.EstimateEnabled && !w.UsageIncomplete {
		v := new(big.Rat).Sub(current, numeric(w.SinceBaselineCost)).FloatString(6)
		w.EstimatedBalance = &v
		current = numeric(v)
	}
	if w.LowThreshold != nil {
		w.Low = current.Cmp(numeric(*w.LowThreshold)) <= 0
	}
	// Forecasts are estimates, and only make sense for comparable USD wallet funds.
	if w.Observation.Currency == "USD" && (w.Observation.Scope == "wallet" || (w.Observation.Scope == "manual" && w.EstimateEnabled)) && !w.Stale && !w.UsageIncomplete && len(w.AccountIDs) > 0 && numeric(w.WeekCost).Sign() > 0 {
		days := new(big.Rat).Quo(new(big.Rat).Mul(current, big.NewRat(7, 1)), numeric(w.WeekCost))
		if days.Sign() < 0 {
			days = new(big.Rat)
		}
		v := days.FloatString(1)
		w.DaysRemaining = &v
	}
}
func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}
