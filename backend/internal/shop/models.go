// Package shop owns third-party subscription purchasing, independently of gateway billing.
package shop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/mail"
	"net/url"
	"strings"
	"time"

	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type Product struct {
	ID                string   `json:"id"`
	Brand             string   `json:"brand"`
	Name              string   `json:"name"`
	Spec              string   `json:"spec"`
	Description       string   `json:"description"`
	Requirements      string   `json:"requirements"`
	PurchaseNotice    string   `json:"purchase_notice"`
	DeliveryHours     int      `json:"delivery_hours"`
	PriceCents        *int64   `json:"price_cents"`
	PaymentURL        string   `json:"payment_url"`
	ReferenceAmount   *float64 `json:"reference_amount"`
	ReferenceCurrency string   `json:"reference_currency"`
	ReferenceURL      string   `json:"reference_url"`
	ReferenceDate     string   `json:"reference_date"`
	Active            bool     `json:"active"`
	Sort              int      `json:"sort"`
	Version           int      `json:"version"`
}

type Event struct {
	ActorID int64     `json:"actor_id"`
	Action  string    `json:"action"`
	Note    string    `json:"note"`
	At      time.Time `json:"at"`
}

type Order struct {
	ID                string     `json:"id"`
	UserID            int64      `json:"user_id"`
	ApplicantEmail    string     `json:"applicant_email"`
	TargetEmail       string     `json:"target_email"`
	ContactEmail      string     `json:"contact_email"`
	Product           Product    `json:"product"`
	AmountCents       int64      `json:"amount_cents"`
	PaymentStatus     string     `json:"payment_status"`
	FulfillmentStatus string     `json:"fulfillment_status"`
	ExternalReference string     `json:"external_reference"`
	Note              string     `json:"note"`
	DeliveryNote      string     `json:"delivery_note"`
	CreatedAt         time.Time  `json:"created_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	ValidUntil        string     `json:"valid_until"`
	Version           int        `json:"version"`
	Events            []Event    `json:"events"`
}

type CreateOrder struct {
	ProductID      string `json:"product_id"`
	ProductVersion int    `json:"product_version"`
	TargetEmail    string `json:"target_email"`
	ContactEmail   string `json:"contact_email"`
	Note           string `json:"note"`
	Accepted       bool   `json:"accepted"`
	RequestKey     string `json:"request_key"`
}

type OrderAction struct {
	Action            string `json:"action"`
	Version           int    `json:"version"`
	Note              string `json:"note"`
	ExternalReference string `json:"external_reference"`
	TargetEmail       string `json:"target_email"`
	ValidUntil        string `json:"valid_until"`
}

func invalid(message string) error  { return appErrors.BadRequest("SHOP_INVALID_INPUT", message) }
func conflict(message string) error { return appErrors.Conflict("SHOP_CONFLICT", message) }

func validLink(value string) bool {
	u, err := url.Parse(value)
	return err == nil && len(value) <= 2000 && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(value, "\r\n")
}

func normalizedEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	if err != nil || len(value) > 254 || address.Address != value || !strings.Contains(value, "@") {
		return "", invalid("请填写有效的邮箱地址")
	}
	return strings.ToLower(value), nil
}

func validateProduct(p *Product) error {
	p.Name, p.Spec = strings.TrimSpace(p.Name), strings.TrimSpace(p.Spec)
	p.PurchaseNotice = strings.TrimSpace(p.PurchaseNotice)
	p.PaymentURL, p.ReferenceURL = strings.TrimSpace(p.PaymentURL), strings.TrimSpace(p.ReferenceURL)
	if (p.Brand != "ChatGPT" && p.Brand != "Anthropic") || p.Name == "" || len(p.Name) > 160 || p.Spec == "" || len(p.Spec) > 160 || len(p.Description) > 3000 || len(p.Requirements) > 3000 || len(p.PurchaseNotice) > 18000 || p.PurchaseNotice == "" {
		return invalid("请核对品牌、套餐名称、周期和购买须知")
	}
	if p.DeliveryHours < 1 || p.DeliveryHours > 720 || p.Sort < 0 || p.Sort > 100000 || (p.PriceCents != nil && (*p.PriceCents <= 0 || *p.PriceCents > 10000000)) {
		return invalid("售价、预计处理时间或排序超出允许范围")
	}
	if p.PaymentURL != "" && !validLink(p.PaymentURL) {
		return invalid("购买链接必须是有效的 HTTPS 地址")
	}
	if p.ReferenceURL != "" && !validLink(p.ReferenceURL) {
		return invalid("官方来源必须是有效的 HTTPS 地址")
	}
	if p.ReferenceAmount != nil {
		_, err := time.Parse(time.DateOnly, p.ReferenceDate)
		if math.IsNaN(*p.ReferenceAmount) || math.IsInf(*p.ReferenceAmount, 0) || *p.ReferenceAmount <= 0 || *p.ReferenceAmount > 1000000 || !validLink(p.ReferenceURL) || err != nil {
			return invalid("参考价格须填写有效金额、官方来源和核对日期")
		}
	}
	switch p.ReferenceCurrency {
	case "USD", "CNY", "EUR", "HKD", "GBP", "JPY":
	default:
		return invalid("请选择有效的参考币种")
	}
	return nil
}

func requestHash(req CreateOrder) string {
	req.RequestKey = ""
	encoded, _ := json.Marshal(req)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func applyAction(order *Order, action OrderAction, admin bool, actor int64, now time.Time) error {
	if action.Version != order.Version {
		return conflict("订单已被更新，请刷新后重试")
	}
	if len(action.Note) > 3000 || len(action.ExternalReference) > 200 {
		return invalid("备注或外站订单号过长")
	}
	paid, pending := order.PaymentStatus == "PAID", order.PaymentStatus == "PENDING"
	switch {
	case action.Action == "cancel" && pending:
		order.PaymentStatus, order.FulfillmentStatus = "CANCELLED", "CLOSED"
	case action.Action == "payment_reference" && !admin && pending:
		if strings.TrimSpace(action.ExternalReference) == "" {
			return invalid("请填写外站订单号")
		}
		order.ExternalReference = strings.TrimSpace(action.ExternalReference)
	case action.Action == "supplement" && !admin && (pending || (paid && (order.FulfillmentStatus == "WAITING" || order.FulfillmentStatus == "NEEDS_INFO"))):
		email, err := normalizedEmail(action.TargetEmail)
		if err != nil {
			return err
		}
		order.TargetEmail, order.Note = email, strings.TrimSpace(action.Note)
	case action.Action == "confirm_paid" && admin && (pending || order.PaymentStatus == "CANCELLED"):
		if strings.TrimSpace(action.ExternalReference) == "" {
			return invalid("人工确认收款须登记外站订单号")
		}
		order.ExternalReference = strings.TrimSpace(action.ExternalReference)
		order.PaymentStatus, order.FulfillmentStatus, order.PaidAt = "PAID", "WAITING", &now
	case action.Action == "needs_info" && admin && paid && (order.FulfillmentStatus == "WAITING" || order.FulfillmentStatus == "PROCESSING" || order.FulfillmentStatus == "NEEDS_INFO"):
		if strings.TrimSpace(action.Note) == "" {
			return invalid("请说明需要补充的信息")
		}
		order.FulfillmentStatus = "NEEDS_INFO"
	case action.Action == "process" && admin && paid && (order.FulfillmentStatus == "WAITING" || order.FulfillmentStatus == "NEEDS_INFO"):
		order.FulfillmentStatus = "PROCESSING"
	case action.Action == "complete" && admin && paid && order.FulfillmentStatus == "PROCESSING":
		if strings.TrimSpace(action.Note) == "" {
			return invalid("请填写实际交付说明")
		}
		if action.ValidUntil != "" {
			until, err := time.Parse(time.RFC3339, action.ValidUntil)
			if err != nil || !until.After(now) {
				return invalid("有效期必须晚于当前时间")
			}
		}
		order.FulfillmentStatus, order.DeliveryNote, order.ValidUntil, order.CompletedAt = "COMPLETED", strings.TrimSpace(action.Note), action.ValidUntil, &now
	case action.Action == "confirm_refunded" && admin && paid:
		if strings.TrimSpace(action.Note) == "" {
			return invalid("请登记已在外站完成退款的实际凭证或流水")
		}
		order.PaymentStatus, order.FulfillmentStatus = "REFUNDED", "CLOSED"
	default:
		return conflict("当前订单状态不允许此操作")
	}
	order.Version++
	order.Events = append(order.Events, Event{ActorID: actor, Action: action.Action, Note: strings.TrimSpace(action.Note), At: now})
	return nil
}
