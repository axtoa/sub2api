package cdkshop

import "time"

type Product struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Spec         string    `json:"spec"`
	Description  string    `json:"description"`
	Instructions string    `json:"instructions"`
	PriceCents   int64     `json:"price_cents"`
	RewardCents  int64     `json:"reward_cents"`
	Active       bool      `json:"active"`
	Sort         int       `json:"sort"`
	Version      int       `json:"version"`
	Stock        int       `json:"stock"`
	CreatedAt    time.Time `json:"created_at"`
}

type Order struct {
	ID             string     `json:"id"`
	UserID         int64      `json:"user_id,omitempty"`
	Email          string     `json:"email"`
	ProductID      string     `json:"product_id"`
	ProductName    string     `json:"product_name"`
	PriceCents     int64      `json:"price_cents"`
	RewardCents    int64      `json:"reward_cents"`
	PaymentStatus  string     `json:"payment_status"`
	DeliveryStatus string     `json:"delivery_status"`
	PayURL         string     `json:"pay_url,omitempty"`
	QRCode         string     `json:"qr_code,omitempty"`
	CDK            string     `json:"cdk,omitempty"`
	RedeemURL      string     `json:"redeem_url"`
	CreatedAt      time.Time  `json:"created_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	RewardStatus   string     `json:"reward_status,omitempty"`
	RewardIssuedAt *time.Time `json:"reward_issued_at,omitempty"`
	RewardIssuedBy int64      `json:"reward_issued_by,omitempty"`
	RewardNote     string     `json:"reward_note,omitempty"`
	GuestAccessKey string     `json:"guest_access_key,omitempty"`
}

type CreateOrderRequest struct {
	ProductID string `json:"product_id" binding:"required,uuid"`
	Email     string `json:"email" binding:"required,email"`
	RequestID string `json:"request_id" binding:"required,uuid"`
}

type CreateOrderResponse struct {
	Order       Order  `json:"order"`
	RecoveryKey string `json:"recovery_key,omitempty"`
}

type PaymentSettings struct {
	Enabled bool   `json:"enabled"`
	APIBase string `json:"api_base"`
	PID     string `json:"pid"`
	PKey    string `json:"pkey,omitempty"`
	Channel string `json:"channel"`
}

type PaymentSettingsView struct {
	Enabled       bool   `json:"enabled"`
	APIBase       string `json:"api_base"`
	PID           string `json:"pid"`
	Channel       string `json:"channel"`
	Configured    bool   `json:"configured"`
	ConfigVersion int    `json:"config_version"`
}

type RewardTask struct {
	OrderID     string     `json:"order_id"`
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	ProductName string     `json:"product_name"`
	AmountCents int64      `json:"amount_cents"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	IssuedAt    *time.Time `json:"issued_at,omitempty"`
	IssuedBy    int64      `json:"issued_by,omitempty"`
	Note        string     `json:"note,omitempty"`
}
