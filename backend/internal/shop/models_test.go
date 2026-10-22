package shop

import (
	"math"
	"testing"
	"time"

	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func productFixture() Product {
	price := int64(12300)
	return Product{ID: "88000000-0000-4000-8000-000000000002", Brand: "ChatGPT", Name: "Plus", Spec: "Monthly", PriceCents: &price, PaymentURL: "https://shop.example.com/plus", PurchaseNotice: "Manual purchasing terms", ReferenceCurrency: "USD", DeliveryHours: 24, Active: true, Sort: 20, Version: 3}
}

func orderFixture() Order {
	return Order{ID: "SOtest", UserID: 42, ApplicantEmail: "member@example.com", TargetEmail: "target@example.com", Product: productFixture(), AmountCents: 12300, PaymentStatus: "PENDING", FulfillmentStatus: "WAITING", Version: 1}
}

func TestValidateProduct(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Product)
	}{
		{"unsupported brand", func(p *Product) { p.Brand = "Gemini" }},
		{"empty terms", func(p *Product) { p.PurchaseNotice = "  " }},
		{"zero price", func(p *Product) { v := int64(0); p.PriceCents = &v }},
		{"too large price", func(p *Product) { v := int64(10000001); p.PriceCents = &v }},
		{"unsafe checkout", func(p *Product) { p.PaymentURL = "javascript:alert(1)" }},
		{"plain http", func(p *Product) { p.PaymentURL = "http://shop.example.com" }},
		{"embedded credentials", func(p *Product) { p.PaymentURL = "https://secret@example.com" }},
		{"newline url", func(p *Product) { p.PaymentURL = "https://example.com/one\ntwo" }},
		{"invalid currency", func(p *Product) { p.ReferenceCurrency = "invalid" }},
		{"missing reference evidence", func(p *Product) { v := 20.0; p.ReferenceAmount = &v }},
		{"nonfinite reference", func(p *Product) { v := math.NaN(); p.ReferenceAmount = &v }},
		{"invalid processing time", func(p *Product) { p.DeliveryHours = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := productFixture()
			tc.edit(&p)
			require.Error(t, validateProduct(&p))
		})
	}
	p := productFixture()
	require.NoError(t, validateProduct(&p))
	p.PriceCents, p.PaymentURL = nil, ""
	require.NoError(t, validateProduct(&p), "an upcoming product can be saved without a checkout")
	v := 20.0
	p.ReferenceAmount, p.ReferenceURL, p.ReferenceDate = &v, "https://chatgpt.com/pricing", "2026-10-22"
	require.NoError(t, validateProduct(&p))
}

func TestEmailValidation(t *testing.T) {
	email, err := normalizedEmail("  Name@Example.com  ")
	require.NoError(t, err)
	require.Equal(t, "name@example.com", email)
	for _, value := range []string{"", "not-email", "Name <name@example.com>", "one@example.com,two@example.com", "name@example.com\r\nBcc:other@example.com"} {
		_, err := normalizedEmail(value)
		require.Error(t, err, value)
	}
}

func TestOrderManualFulfillmentLifecycle(t *testing.T) {
	order := orderFixture()
	now := time.Now().UTC()
	apply := func(action string, admin bool, note, reference string) {
		t.Helper()
		require.NoError(t, applyAction(&order, OrderAction{Action: action, Version: order.Version, Note: note, ExternalReference: reference}, admin, 42, now))
	}
	apply("payment_reference", false, "", "EXTERNAL-123")
	require.Equal(t, "PENDING", order.PaymentStatus, "a submitted reference is not proof of payment")
	apply("confirm_paid", true, "Verified amount", "EXTERNAL-123")
	require.Equal(t, "PAID", order.PaymentStatus)
	require.NotNil(t, order.PaidAt)
	apply("needs_info", true, "Please check your account", "")
	require.Equal(t, "NEEDS_INFO", order.FulfillmentStatus)
	require.NoError(t, applyAction(&order, OrderAction{Action: "supplement", Version: order.Version, TargetEmail: "Other@Example.com", Note: "Corrected"}, false, 42, now))
	require.Equal(t, "other@example.com", order.TargetEmail)
	apply("process", true, "", "")
	apply("complete", true, "Activated official subscription", "")
	require.Equal(t, "COMPLETED", order.FulfillmentStatus)
	require.NotNil(t, order.CompletedAt)
	require.Equal(t, "Activated official subscription", order.DeliveryNote)
	require.Equal(t, 6, len(order.Events))
	apply("confirm_refunded", true, "External refund receipt REF-1", "")
	require.Equal(t, "REFUNDED", order.PaymentStatus)
	require.Equal(t, "CLOSED", order.FulfillmentStatus)
}

func TestOrderRejectsIllegalActionsWithoutMutating(t *testing.T) {
	for _, tc := range []struct {
		action, payment, fulfillment, note, ref, expiry string
		admin                                           bool
	}{
		{action: "confirm_paid", ref: "receipt"},
		{action: "complete", payment: "PAID", fulfillment: "PROCESSING", note: "done"},
		{action: "confirm_refunded", payment: "PAID", note: "receipt"},
		{action: "process", admin: true},
		{action: "complete", payment: "PAID", fulfillment: "WAITING", admin: true, note: "done"},
		{action: "complete", payment: "PAID", fulfillment: "PROCESSING", admin: true},
		{action: "confirm_paid", admin: true},
		{action: "cancel", payment: "PAID"},
		{action: "supplement", payment: "PAID", fulfillment: "PROCESSING"},
		{action: "payment_reference"},
		{action: "needs_info", payment: "PAID", fulfillment: "WAITING", admin: true},
		{action: "complete", payment: "PAID", fulfillment: "PROCESSING", admin: true, note: "done", expiry: "2000-01-01T00:00:00Z"},
		{action: "confirm_refunded", payment: "PAID", admin: true},
	} {
		t.Run(tc.action+tc.payment+tc.fulfillment+tc.expiry+tc.note+tc.ref, func(t *testing.T) {
			o := orderFixture()
			if tc.payment != "" {
				o.PaymentStatus = tc.payment
			}
			if tc.fulfillment != "" {
				o.FulfillmentStatus = tc.fulfillment
			}
			before := o
			err := applyAction(&o, OrderAction{Action: tc.action, Version: 1, Note: tc.note, ExternalReference: tc.ref, ValidUntil: tc.expiry}, tc.admin, 42, time.Now())
			require.Error(t, err)
			require.Equal(t, before, o)
		})
	}
	o := orderFixture()
	require.Equal(t, 409, appErrors.Code(applyAction(&o, OrderAction{Action: "cancel", Version: 0}, false, 42, time.Now())))
}

func TestCancellationCanBeReconciledAfterLateExternalPayment(t *testing.T) {
	o := orderFixture()
	now := time.Now()
	require.NoError(t, applyAction(&o, OrderAction{Action: "cancel", Version: 1}, false, 42, now))
	require.NoError(t, applyAction(&o, OrderAction{Action: "confirm_paid", Version: 2, ExternalReference: "late-receipt"}, true, 1, now))
	require.Equal(t, "PAID", o.PaymentStatus)
	require.Equal(t, "WAITING", o.FulfillmentStatus)
}
