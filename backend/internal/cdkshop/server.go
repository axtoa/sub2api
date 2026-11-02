package cdkshop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type Server struct {
	db          *sql.DB
	crypto      *Crypto
	pay         payment.Provider
	publicURL   string
	tokenSecret []byte
}

//go:embed schema.sql
var schemaSource string

func OpenServer(ctx context.Context, dsn, encryptionKey, authSecret, payBase, payPID, payKey, publicURL string) (*Server, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	schema, err := schemaSQL()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err = db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cdkshop schema: %w", err)
	}
	crypto, err := NewCrypto(encryptionKey)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	pay, err := provider.NewEasyPay("cdkshop", map[string]string{"apiBase": payBase, "pid": payPID, "pkey": payKey, "notifyUrl": strings.TrimRight(publicURL, "/") + "/api/v1/cdkshop/payment/notify", "returnUrl": strings.TrimRight(publicURL, "/") + "/shop/complete"})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if len(authSecret) < 32 {
		_ = db.Close()
		return nil, fmt.Errorf("CDKSHOP_AUTH_SECRET must be at least 32 bytes")
	}
	return &Server{db: db, crypto: crypto, pay: pay, publicURL: strings.TrimRight(publicURL, "/"), tokenSecret: []byte(authSecret)}, nil
}

// schemaSQL is kept in a separate file so this service never invokes the core migration runner.
func schemaSQL() (string, error) { return schemaSource, nil }

func (s *Server) Close() error { return s.db.Close() }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/v1/cdkshop/products", s.products)
	mux.HandleFunc("GET /api/v1/cdkshop/status", s.publicStatus)
	mux.HandleFunc("POST /api/v1/cdkshop/orders", s.createOrder)
	mux.HandleFunc("GET /api/v1/cdkshop/orders/{id}", s.getOrder)
	mux.HandleFunc("POST /api/v1/cdkshop/orders/{id}/payment", s.retryPayment)
	mux.HandleFunc("POST /api/v1/cdkshop/payment/notify", s.notify)
	mux.HandleFunc("GET /api/v1/cdkshop/payment/notify", s.notify)
	mux.HandleFunc("GET /api/v1/cdkshop/admin/products", s.adminProducts)
	mux.HandleFunc("POST /api/v1/cdkshop/admin/products", s.adminCreateProduct)
	mux.HandleFunc("POST /api/v1/cdkshop/admin/products/{id}/cdks", s.adminImportCDKs)
	mux.HandleFunc("GET /api/v1/cdkshop/admin/rewards", s.adminRewards)
	mux.HandleFunc("POST /api/v1/cdkshop/admin/rewards/{id}/issue", s.adminIssueReward)
	mux.HandleFunc("GET /api/v1/cdkshop/admin/settings", s.adminSettings)
	mux.HandleFunc("PUT /api/v1/cdkshop/admin/settings", s.setAdminSettings)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) shopEnabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT value='true' FROM cdkshop_settings WHERE key='store_enabled'),false)`).Scan(&enabled)
	return enabled, err
}
func (s *Server) publicStatus(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.shopEnabled(r.Context())
	if err != nil {
		writeError(w, 500, "store_status_unavailable")
		return
	}
	writeJSON(w, 200, map[string]bool{"enabled": enabled})
}
func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	enabled, err := s.shopEnabled(r.Context())
	if err != nil {
		writeError(w, 500, "store_status_unavailable")
		return
	}
	writeJSON(w, 200, map[string]bool{"enabled": enabled})
}
func (s *Server) setAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body) != nil || body.Enabled == nil {
		writeError(w, 400, "invalid_store_setting")
		return
	}
	_, err := s.db.ExecContext(r.Context(), `INSERT INTO cdkshop_settings(key,value,updated_at) VALUES('store_enabled',$1,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, strconv.FormatBool(*body.Enabled))
	if err != nil {
		writeError(w, 500, "store_setting_save_failed")
		return
	}
	writeJSON(w, 200, map[string]bool{"enabled": *body.Enabled})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims, err := VerifyToken(s.authSecret(), bearer(r), time.Now())
	if err != nil || claims.Role != "admin" {
		writeError(w, 403, "admin_required")
		return false
	}
	return true
}

func (s *Server) adminProducts(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT p.id,p.name,p.spec,p.description,p.instructions,p.price_cents,p.reward_cents,p.active,p.sort_order,p.version,p.created_at,COUNT(c.id) FROM cdkshop_products p LEFT JOIN cdkshop_cdks c ON c.product_id=p.id AND c.status='available' GROUP BY p.id ORDER BY p.sort_order,p.created_at`)
	if err != nil {
		writeError(w, 500, "catalog_unavailable")
		return
	}
	defer func() { _ = rows.Close() }()
	items := []Product{}
	for rows.Next() {
		var p Product
		if rows.Scan(&p.ID, &p.Name, &p.Spec, &p.Description, &p.Instructions, &p.PriceCents, &p.RewardCents, &p.Active, &p.Sort, &p.Version, &p.CreatedAt, &p.Stock) != nil {
			writeError(w, 500, "catalog_unavailable")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) adminCreateProduct(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var p Product
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&p) != nil || strings.TrimSpace(p.Name) == "" || p.PriceCents < 1 || p.PriceCents > 10000000 || p.RewardCents < 0 || p.RewardCents > 10000000 {
		writeError(w, 400, "invalid_product")
		return
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	} else if !uuidValidate(p.ID) {
		writeError(w, 400, "invalid_product_id")
		return
	}
	if p.Spec == "" {
		p.Spec = "兑换码"
	}
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO cdkshop_products(id,name,spec,description,instructions,price_cents,reward_cents,active,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.ID, p.Name, p.Spec, p.Description, p.Instructions, p.PriceCents, p.RewardCents, p.Active, p.Sort); err != nil {
		writeError(w, 409, "product_create_failed")
		return
	}
	writeJSON(w, 201, p)
}

func (s *Server) adminImportCDKs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	productID := r.PathValue("id")
	var exists bool
	if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM cdkshop_products WHERE id=$1)`, productID).Scan(&exists); err != nil || !exists {
		writeError(w, 404, "product_not_found")
		return
	}
	var body struct {
		Codes []string `json:"codes"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&body) != nil || len(body.Codes) == 0 || len(body.Codes) > 1000 {
		writeError(w, 400, "invalid_cdk_batch")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "import_failed")
		return
	}
	defer func() { _ = tx.Rollback() }()
	count := 0
	for _, raw := range body.Codes {
		code := strings.TrimSpace(raw)
		if len(code) < 4 || len(code) > 512 {
			continue
		}
		encrypted, e := s.crypto.Encrypt(code)
		if e != nil {
			writeError(w, 500, "import_failed")
			return
		}
		result, e := tx.ExecContext(r.Context(), `INSERT INTO cdkshop_cdks(product_id,encrypted_code,fingerprint) VALUES($1,$2,$3) ON CONFLICT(fingerprint) DO NOTHING`, productID, encrypted, s.crypto.Fingerprint(code))
		if e != nil {
			writeError(w, 400, "product_not_found")
			return
		}
		inserted, _ := result.RowsAffected()
		count += int(inserted)
	}
	if tx.Commit() != nil {
		writeError(w, 500, "import_failed")
		return
	}
	writeJSON(w, 200, map[string]int{"imported": count})
}

func (s *Server) adminRewards(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT order_id,user_id,email,amount_cents,status,created_at,issued_at,COALESCE(issued_by,0),note FROM cdkshop_reward_tasks ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		writeError(w, 500, "rewards_unavailable")
		return
	}
	defer func() { _ = rows.Close() }()
	tasks := []RewardTask{}
	for rows.Next() {
		var t RewardTask
		if rows.Scan(&t.OrderID, &t.UserID, &t.Email, &t.AmountCents, &t.Status, &t.CreatedAt, &t.IssuedAt, &t.IssuedBy, &t.Note) != nil {
			writeError(w, 500, "rewards_unavailable")
			return
		}
		tasks = append(tasks, t)
	}
	writeJSON(w, 200, map[string]any{"items": tasks})
}

func (s *Server) adminIssueReward(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body) != nil || len(body.Note) > 1000 {
		writeError(w, 400, "invalid_note")
		return
	}
	claims, _ := VerifyToken(s.authSecret(), bearer(r), time.Now())
	result, err := s.db.ExecContext(r.Context(), `UPDATE cdkshop_reward_tasks SET status='issued',issued_at=NOW(),issued_by=$2,note=$3 WHERE order_id=$1 AND status='pending'`, r.PathValue("id"), claims.Subject, body.Note)
	if err != nil {
		writeError(w, 500, "reward_update_failed")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "reward_not_found")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "issued"})
}

func (s *Server) products(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT p.id,p.name,p.spec,p.description,p.instructions,p.price_cents,p.reward_cents,p.active,p.sort_order,p.version,p.created_at,COUNT(c.id) FROM cdkshop_products p LEFT JOIN cdkshop_cdks c ON c.product_id=p.id AND c.status='available' WHERE p.active=true GROUP BY p.id ORDER BY p.sort_order,p.created_at`)
	if err != nil {
		writeError(w, 500, "catalog_unavailable")
		return
	}
	defer func() { _ = rows.Close() }()
	items := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err = rows.Scan(&p.ID, &p.Name, &p.Spec, &p.Description, &p.Instructions, &p.PriceCents, &p.RewardCents, &p.Active, &p.Sort, &p.Version, &p.CreatedAt, &p.Stock); err != nil {
			writeError(w, 500, "catalog_unavailable")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil || !uuidValidate(req.ProductID) || !uuidValidate(req.RequestID) || !validEmail(req.Email) {
		writeError(w, 400, "invalid_order")
		return
	}
	var userID int64
	if token := bearer(r); token != "" {
		claims, err := VerifyToken(s.authSecret(), token, time.Now())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "shop_session_expired")
			return
		}
		userID = claims.Subject
	}
	// The reserve and order are committed together. Every successful payment therefore owns a CDK.
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "order_unavailable")
		return
	}
	defer func() { _ = tx.Rollback() }()
	var existing Order
	var recoveryCipher string
	err = tx.QueryRowContext(r.Context(), `SELECT id,email,product_id,product_name,price_cents,reward_cents,payment_status,delivery_status,recovery_secret,created_at,pay_url,qr_code,COALESCE(user_id,0) FROM cdkshop_orders WHERE request_id=$1`, req.RequestID).Scan(&existing.ID, &existing.Email, &existing.ProductID, &existing.ProductName, &existing.PriceCents, &existing.RewardCents, &existing.PaymentStatus, &existing.DeliveryStatus, &recoveryCipher, &existing.CreatedAt, &existing.PayURL, &existing.QRCode, &existing.UserID)
	if err == nil {
		if !strings.EqualFold(existing.Email, strings.TrimSpace(req.Email)) || existing.UserID != userID || existing.ProductID != req.ProductID {
			writeError(w, 409, "request_id_conflict")
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, 500, "order_unavailable")
			return
		}
		recovery, _ := s.crypto.Decrypt(recoveryCipher)
		writeJSON(w, 200, CreateOrderResponse{Order: existing, RecoveryKey: recovery})
		return
	}
	if err != sql.ErrNoRows {
		writeError(w, 500, "order_unavailable")
		return
	}
	// Reuse the order transaction's connection so concurrent checkouts cannot
	// exhaust the pool while each waits for a second connection to read settings.
	var enabled bool
	err = tx.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT value='true' FROM cdkshop_settings WHERE key='store_enabled'),false)`).Scan(&enabled)
	if err != nil {
		writeError(w, 500, "store_status_unavailable")
		return
	}
	if !enabled {
		writeError(w, 403, "store_paused")
		return
	}
	var p Product
	err = tx.QueryRowContext(r.Context(), `SELECT id,name,spec,description,instructions,price_cents,reward_cents,active,sort_order,version,created_at FROM cdkshop_products WHERE id=$1 FOR UPDATE`, req.ProductID).Scan(&p.ID, &p.Name, &p.Spec, &p.Description, &p.Instructions, &p.PriceCents, &p.RewardCents, &p.Active, &p.Sort, &p.Version, &p.CreatedAt)
	if err != nil || !p.Active {
		writeError(w, 404, "product_unavailable")
		return
	}
	var key [32]byte
	if _, err = rand.Read(key[:]); err != nil {
		writeError(w, 500, "order_unavailable")
		return
	}
	recovery := base64.RawURLEncoding.EncodeToString(key[:])
	hash := sha256.Sum256([]byte(recovery))
	recoveryCipher, err = s.crypto.Encrypt(recovery)
	if err != nil {
		writeError(w, 500, "order_unavailable")
		return
	}
	orderID := uuid.NewString()
	var cdkID int64
	err = tx.QueryRowContext(r.Context(), `UPDATE cdkshop_cdks SET status='reserved',order_id=$1 WHERE id=(SELECT id FROM cdkshop_cdks WHERE product_id=$2 AND status='available' ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id`, orderID, p.ID).Scan(&cdkID)
	if err != nil {
		writeError(w, 409, "out_of_stock")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO cdkshop_orders(id,request_id,user_id,email,recovery_hash,recovery_secret,product_id,product_name,price_cents,reward_cents,cdk_id) VALUES($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,$9,$10,$11)`, orderID, req.RequestID, userID, strings.ToLower(strings.TrimSpace(req.Email)), base64.RawURLEncoding.EncodeToString(hash[:]), recoveryCipher, p.ID, p.Name, p.PriceCents, p.RewardCents, cdkID)
	if err != nil {
		writeError(w, 500, "order_unavailable")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "order_unavailable")
		return
	}
	amount := fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100)
	resp, err := s.pay.CreatePayment(r.Context(), payment.CreatePaymentRequest{OrderID: orderID, Amount: amount, PaymentType: "alipay", Subject: p.Name, NotifyURL: s.publicURL + "/api/v1/cdkshop/payment/notify", ReturnURL: s.publicURL + "/shop?order=" + url.QueryEscape(orderID), ClientIP: clientIP(r), IsMobile: strings.Contains(strings.ToLower(r.UserAgent()), "mobile")})
	if err != nil {
		// The provider may have accepted the request before the network timed out.
		// Keep this order and its CDK reserved so a late payment cannot lose delivery.
		writeJSON(w, 201, CreateOrderResponse{Order: Order{ID: orderID, Email: req.Email, ProductID: p.ID, ProductName: p.Name, PriceCents: p.PriceCents, PaymentStatus: "pending", DeliveryStatus: "pending", RedeemURL: "https://shop.aisale.best/recharge", CreatedAt: time.Now()}, RecoveryKey: recovery})
		return
	}
	_, _ = s.db.ExecContext(r.Context(), `UPDATE cdkshop_orders SET pay_url=$2,qr_code=$3,trade_no=$4 WHERE id=$1`, orderID, resp.PayURL, resp.QRCode, resp.TradeNo)
	writeJSON(w, 201, CreateOrderResponse{Order: Order{ID: orderID, Email: req.Email, ProductID: p.ID, ProductName: p.Name, PriceCents: p.PriceCents, PaymentStatus: "pending", DeliveryStatus: "pending", PayURL: resp.PayURL, QRCode: resp.QRCode, RedeemURL: "https://shop.aisale.best/recharge", CreatedAt: time.Now()}, RecoveryKey: recovery})
}

func (s *Server) retryPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var o Order
	var recoveryHash string
	err := s.db.QueryRowContext(r.Context(), `SELECT id,COALESCE(user_id,0),email,product_id,product_name,price_cents,payment_status,delivery_status,recovery_hash,pay_url FROM cdkshop_orders WHERE id=$1`, id).Scan(&o.ID, &o.UserID, &o.Email, &o.ProductID, &o.ProductName, &o.PriceCents, &o.PaymentStatus, &o.DeliveryStatus, &recoveryHash, &o.PayURL)
	if err != nil {
		writeError(w, 404, "order_not_found")
		return
	}
	allowed := false
	if token := bearer(r); token != "" {
		if c, e := VerifyToken(s.authSecret(), token, time.Now()); e == nil && o.UserID > 0 && c.Subject == o.UserID {
			allowed = true
		}
	}
	if !allowed {
		key := strings.TrimSpace(r.Header.Get("X-Order-Recovery-Key"))
		sum := sha256.Sum256([]byte(key))
		allowed = key != "" && base64.RawURLEncoding.EncodeToString(sum[:]) == recoveryHash
	}
	if !allowed {
		writeError(w, 403, "order_access_denied")
		return
	}
	if o.PaymentStatus != "pending" || o.PayURL != "" {
		writeError(w, 409, "payment_already_started")
		return
	}
	enabled, err := s.shopEnabled(r.Context())
	if err != nil {
		writeError(w, 500, "store_status_unavailable")
		return
	}
	if !enabled {
		writeError(w, 403, "store_paused")
		return
	}
	amount := fmt.Sprintf("%d.%02d", o.PriceCents/100, o.PriceCents%100)
	resp, err := s.pay.CreatePayment(r.Context(), payment.CreatePaymentRequest{OrderID: o.ID, Amount: amount, PaymentType: "alipay", Subject: o.ProductName, NotifyURL: s.publicURL + "/api/v1/cdkshop/payment/notify", ReturnURL: s.publicURL + "/shop?order=" + url.QueryEscape(o.ID), ClientIP: clientIP(r), IsMobile: strings.Contains(strings.ToLower(r.UserAgent()), "mobile")})
	if err != nil {
		writeError(w, 502, "payment_unavailable")
		return
	}
	_, err = s.db.ExecContext(r.Context(), `UPDATE cdkshop_orders SET pay_url=$2,qr_code=$3,trade_no=$4 WHERE id=$1 AND payment_status='pending' AND pay_url=''`, o.ID, resp.PayURL, resp.QRCode, resp.TradeNo)
	if err != nil {
		writeError(w, 500, "order_unavailable")
		return
	}
	o.PayURL = resp.PayURL
	o.QRCode = resp.QRCode
	writeJSON(w, 200, o)
}

func (s *Server) authSecret() []byte { return s.tokenSecret }

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var key string
	var cdkCipher sql.NullString
	var o Order
	err := s.db.QueryRowContext(r.Context(), `SELECT o.id,COALESCE(o.user_id,0),o.email,o.product_id,o.product_name,o.price_cents,o.reward_cents,o.payment_status,o.delivery_status,o.pay_url,o.qr_code,o.created_at,o.paid_at,COALESCE(c.encrypted_code,'') FROM cdkshop_orders o LEFT JOIN cdkshop_cdks c ON c.id=o.cdk_id WHERE o.id=$1`, id).Scan(&o.ID, &o.UserID, &o.Email, &o.ProductID, &o.ProductName, &o.PriceCents, &o.RewardCents, &o.PaymentStatus, &o.DeliveryStatus, &o.PayURL, &o.QRCode, &o.CreatedAt, &o.PaidAt, &cdkCipher)
	if err != nil {
		writeError(w, 404, "order_not_found")
		return
	}
	if tok := bearer(r); tok != "" {
		claims, e := VerifyToken(s.authSecret(), tok, time.Now())
		if e == nil && claims.Subject == o.UserID && o.UserID > 0 {
			if o.DeliveryStatus == "delivered" {
				o.CDK, _ = s.crypto.Decrypt(cdkCipher.String)
			}
			writeJSON(w, 200, o)
			return
		}
	}
	key = strings.TrimSpace(r.Header.Get("X-Order-Recovery-Key"))
	hash := sha256.Sum256([]byte(key))
	var expected string
	if key == "" || s.db.QueryRowContext(r.Context(), `SELECT recovery_hash FROM cdkshop_orders WHERE id=$1`, id).Scan(&expected) != nil || expected != base64.RawURLEncoding.EncodeToString(hash[:]) {
		writeError(w, 403, "order_access_denied")
		return
	}
	if o.DeliveryStatus == "delivered" {
		o.CDK, _ = s.crypto.Decrypt(cdkCipher.String)
	}
	writeJSON(w, 200, o)
}

func (s *Server) notify(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
	if r.Method == http.MethodGet {
		if len(r.URL.RawQuery) > 32<<10 {
			http.Error(w, "fail", http.StatusRequestEntityTooLarge)
			return
		}
		raw = []byte(r.URL.RawQuery)
	}
	if err != nil {
		http.Error(w, "fail", 400)
		return
	}
	n, err := s.pay.VerifyNotification(r.Context(), string(raw), nil)
	if err != nil || n == nil || n.Status != payment.ProviderStatusSuccess {
		http.Error(w, "fail", 400)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "fail", 500)
		return
	}
	defer func() { _ = tx.Rollback() }()
	var price int64
	var userID sql.NullInt64
	var reward int64
	var cdkID int64
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT price_cents,user_id,reward_cents,cdk_id,payment_status FROM cdkshop_orders WHERE id=$1 FOR UPDATE`, n.OrderID).Scan(&price, &userID, &reward, &cdkID, &status)
	if err != nil || int64(n.Amount*100+0.5) != price {
		http.Error(w, "fail", 400)
		return
	}
	if status != "paid" {
		var code string
		err = tx.QueryRowContext(r.Context(), `SELECT encrypted_code FROM cdkshop_cdks WHERE id=$1 AND order_id=$2 AND status='reserved' FOR UPDATE`, cdkID, n.OrderID).Scan(&code)
		if err != nil {
			http.Error(w, "fail", http.StatusConflict)
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE cdkshop_orders SET payment_status='paid',delivery_status='delivered',paid_at=NOW() WHERE id=$1`, n.OrderID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE cdkshop_cdks SET status='delivered' WHERE id=$1`, cdkID)
		}
		if err == nil && userID.Valid && reward > 0 {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO cdkshop_reward_tasks(order_id,user_id,email,amount_cents) SELECT id,user_id,email,reward_cents FROM cdkshop_orders WHERE id=$1 ON CONFLICT(order_id) DO NOTHING`, n.OrderID)
		}
		if err != nil {
			http.Error(w, "fail", 500)
			return
		}
	}
	if tx.Commit() != nil {
		http.Error(w, "fail", 500)
		return
	}
	_, _ = fmt.Fprint(w, "success")
}

func bearer(r *http.Request) string {
	p := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(p) == 2 && strings.EqualFold(p[0], "Bearer") {
		return strings.TrimSpace(p[1])
	}
	return ""
}
func uuidValidate(v string) bool { _, err := uuid.Parse(v); return err == nil }
func validEmail(v string) bool {
	v = strings.TrimSpace(v)
	a, err := mail.ParseAddress(v)
	at := strings.LastIndexByte(v, '@')
	return err == nil && a.Address == v && at > 0 && !strings.ContainsAny(v[at+1:], "/?#:&%") && len(v) <= 254
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
