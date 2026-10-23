// Package ledger owns manual cash records and resource renewals, not gateway billing.
package ledger

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type Entry struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Amount       string    `json:"amount"`
	Currency     string    `json:"currency"`
	Category     string    `json:"category"`
	OccurredOn   string    `json:"occurred_on"`
	Counterparty string    `json:"counterparty"`
	Note         string    `json:"note"`
	ResourceID   *string   `json:"resource_id"`
	Voided       bool      `json:"voided"`
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
}

type Resource struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	Provider      string    `json:"provider"`
	ExpiresAt     time.Time `json:"expires_at"`
	RenewalAmount *string   `json:"renewal_amount"`
	Currency      string    `json:"currency"`
	ManagementURL string    `json:"management_url"`
	Note          string    `json:"note"`
	Active        bool      `json:"active"`
	Version       int       `json:"version"`
}

type EntryInput struct {
	Kind         string         `json:"kind"`
	Amount       string         `json:"amount"`
	Currency     string         `json:"currency"`
	Category     string         `json:"category"`
	OccurredOn   string         `json:"occurred_on"`
	Counterparty string         `json:"counterparty"`
	Note         string         `json:"note"`
	ResourceID   *string        `json:"resource_id"`
	NewResource  *ResourceInput `json:"new_resource,omitempty"`
	Version      int            `json:"version"`
	RequestKey   string         `json:"request_key"`
}

type ResourceInput struct {
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	Provider      string    `json:"provider"`
	ExpiresAt     time.Time `json:"expires_at"`
	RenewalAmount *string   `json:"renewal_amount"`
	Currency      string    `json:"currency"`
	ManagementURL string    `json:"management_url"`
	Note          string    `json:"note"`
	Active        bool      `json:"active"`
	Version       int       `json:"version"`
	RequestKey    string    `json:"request_key"`
}

type RenewalInput struct {
	Version       int       `json:"version"`
	ExpiresAt     time.Time `json:"expires_at"`
	RecordExpense bool      `json:"record_expense"`
	Amount        string    `json:"amount"`
	Currency      string    `json:"currency"`
	OccurredOn    string    `json:"occurred_on"`
	Note          string    `json:"note"`
	RequestKey    string    `json:"request_key"`
}

type Settings struct {
	EmailEnabled bool     `json:"email_enabled"`
	Recipients   []string `json:"recipients"`
	ReminderDays []int    `json:"reminder_days"`
	Version      int      `json:"version"`
}

type Total struct {
	Currency string `json:"currency"`
	Income   string `json:"income"`
	Expense  string `json:"expense"`
	Gift     string `json:"gift"`
	Net      string `json:"net"`
}

type Event struct {
	ActorID int64     `json:"actor_id"`
	Action  string    `json:"action"`
	Before  any       `json:"before"`
	After   any       `json:"after"`
	At      time.Time `json:"at"`
}

func invalid(msg string) error { return apperrors.BadRequest("LEDGER_INVALID", msg) }
func conflict() error {
	return apperrors.Conflict("LEDGER_CONFLICT", "记录已更新或请求重复，请刷新后重试")
}

var moneyPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?$`)

// Integer micro-units avoid floating-point changes to manually entered money.
func normalizeAmount(value string) (string, error) {
	if !moneyPattern.MatchString(value) {
		return "", invalid("金额须大于零，最多六位小数")
	}
	parts := strings.SplitN(value, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	units, _ := strconv.ParseInt(fraction, 10, 64)
	units += whole * 1000000
	if units <= 0 {
		return "", invalid("金额必须大于零")
	}
	return fmt.Sprintf("%d.%06d", units/1000000, units%1000000), nil
}

func validCurrency(value string, gift bool) bool {
	switch value {
	case "CNY", "USD", "EUR", "HKD", "GBP", "JPY":
		return true
	case "CREDIT":
		return gift
	}
	return false
}

func validDate(value string) bool {
	d, err := time.Parse(time.DateOnly, value)
	return err == nil && d.Year() >= 1900 && d.Year() <= 2200 && d.Format(time.DateOnly) == value
}

func (e *EntryInput) validate() error {
	if e.Kind != "income" && e.Kind != "expense" && e.Kind != "gift" {
		return invalid("收支类型无效")
	}
	var err error
	e.Amount, err = normalizeAmount(e.Amount)
	if err != nil {
		return err
	}
	e.Category, e.Counterparty, e.Note = strings.TrimSpace(e.Category), strings.TrimSpace(e.Counterparty), strings.TrimSpace(e.Note)
	if !validCurrency(e.Currency, e.Kind == "gift") || !validDate(e.OccurredOn) || e.Category == "" || utf8.RuneCountInString(e.Category) > 80 || utf8.RuneCountInString(e.Counterparty) > 160 || utf8.RuneCountInString(e.Note) > 3000 {
		return invalid("请核对币种、日期、分类和备注长度")
	}
	if e.ResourceID != nil {
		if _, err := uuid.Parse(*e.ResourceID); err != nil {
			return invalid("关联资源无效")
		}
	}
	if e.NewResource != nil {
		if e.ResourceID != nil || e.Kind != "expense" {
			return invalid("仅支出可以同时建立新资源")
		}
		return e.NewResource.validate()
	}
	return nil
}

func (r *ResourceInput) validate() error {
	r.Name, r.Provider, r.Note = strings.TrimSpace(r.Name), strings.TrimSpace(r.Provider), strings.TrimSpace(r.Note)
	r.ManagementURL = strings.TrimSpace(r.ManagementURL)
	if r.Name == "" || utf8.RuneCountInString(r.Name) > 160 || utf8.RuneCountInString(r.Provider) > 160 || utf8.RuneCountInString(r.Note) > 3000 || (r.Kind != "domain" && r.Kind != "server" && r.Kind != "other") || !validCurrency(r.Currency, false) || r.ExpiresAt.IsZero() || r.ExpiresAt.Year() < 1900 || r.ExpiresAt.Year() > 2200 {
		return invalid("请核对资源名称、类型、到期时间和币种")
	}
	if r.RenewalAmount != nil {
		amount, err := normalizeAmount(*r.RenewalAmount)
		if err != nil {
			return err
		}
		r.RenewalAmount = &amount
	}
	if r.ManagementURL != "" {
		u, err := url.Parse(r.ManagementURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(r.ManagementURL) > 2000 || strings.ContainsAny(r.ManagementURL, "\r\n") {
			return invalid("管理入口须为 HTTPS 链接，不可包含登录凭据")
		}
	}
	return nil
}

func (s *Settings) validate() error {
	if s.Recipients == nil {
		s.Recipients = []string{}
	}
	if len(s.Recipients) > 5 || len(s.ReminderDays) == 0 || len(s.ReminderDays) > 10 {
		return invalid("最多五个提醒邮箱，至少设置一个提前提醒天数")
	}
	seen := map[string]bool{}
	for i, email := range s.Recipients {
		email = strings.ToLower(strings.TrimSpace(email))
		addr, err := mail.ParseAddress(email)
		if err != nil || addr.Address != email || len(email) > 254 || seen[email] {
			return invalid("提醒邮箱无效或重复")
		}
		seen[email], s.Recipients[i] = true, email
	}
	if s.EmailEnabled && len(s.Recipients) == 0 {
		return invalid("请先填写提醒邮箱")
	}
	days := map[int]bool{}
	for _, d := range s.ReminderDays {
		if d < 1 || d > 365 || days[d] {
			return invalid("提前提醒天数须为 1 至 365，且不重复")
		}
		days[d] = true
	}
	sort.Sort(sort.Reverse(sort.IntSlice(s.ReminderDays)))
	return nil
}

func thresholdFor(expiry, now time.Time, days []int, loc *time.Location) (int, bool) {
	if !expiry.After(now) {
		return 0, true
	}
	e, n := expiry.In(loc), now.In(loc)
	// Count calendar dates rather than 24-hour periods, including DST transitions.
	date := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
	remaining := int(date(e).Sub(date(n)).Hours() / 24)
	threshold := 366
	for _, d := range days {
		if remaining <= d && d < threshold {
			threshold = d
		}
	}
	return threshold, threshold != 366
}
