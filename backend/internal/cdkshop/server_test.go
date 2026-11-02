package cdkshop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestCreateOrderRejectsWhenStoreIsPaused(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Server{db: db}
	requestID := "11111111-1111-4111-8111-111111111111"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id,email,product_id,product_name,price_cents,reward_cents,payment_status,delivery_status,recovery_secret,created_at,pay_url,qr_code,COALESCE\(user_id,0\) FROM cdkshop_orders WHERE request_id=\$1`).
		WithArgs(requestID).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT COALESCE\(\(SELECT value='true' FROM cdkshop_settings WHERE key='store_enabled'\),false\)`).
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
	mock.ExpectRollback()
	body := `{"product_id":"22222222-2222-4222-8222-222222222222","email":"buyer@example.com","request_id":"` + requestID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cdkshop/orders", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "store_paused") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOrderQueryUsesQualifiedColumnsAndAccountAuthorization(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	secret := []byte("0123456789abcdef0123456789abcdef")
	token, err := SignToken(secret, 17, "user", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, tokenSecret: secret}
	query := `SELECT o.id,COALESCE(o.user_id,0),o.email,o.product_id,o.product_name,o.price_cents,o.reward_cents,o.payment_status,o.delivery_status,o.pay_url,o.qr_code,o.created_at,o.paid_at,COALESCE(c.encrypted_code,'') FROM cdkshop_orders o LEFT JOIN cdkshop_cdks c ON c.id=o.cdk_id WHERE o.id=$1`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("order-1").WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "email", "product_id", "product_name", "price_cents", "reward_cents", "payment_status", "delivery_status", "pay_url", "qr_code", "created_at", "paid_at", "encrypted_code"}).AddRow("order-1", 17, "buyer@example.com", "product-1", "Plus", 9900, 500, "pending", "pending", "", "", time.Now(), nil, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cdkshop/orders/order-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private order responses must not be cached")
	}
	var order Order
	if err := json.Unmarshal(w.Body.Bytes(), &order); err != nil || order.ID != "order-1" {
		t.Fatalf("unexpected order: %+v, %v", order, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOrderDoesNotDowngradeInvalidLoginToGuest(t *testing.T) {
	s := &Server{tokenSecret: []byte("0123456789abcdef0123456789abcdef")}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cdkshop/orders", strings.NewReader(`{"product_id":"22222222-2222-4222-8222-222222222222","email":"buyer@example.com","request_id":"11111111-1111-4111-8111-111111111111"}`))
	req.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

type notificationVerifier struct {
	payment.Provider
	raw string
}

func (p *notificationVerifier) VerifyNotification(_ context.Context, raw string, _ map[string]string) (*payment.PaymentNotification, error) {
	p.raw = raw
	return nil, fmt.Errorf("invalid signature")
}

func TestCallbackVerifiesBothGETAndPOSTWithoutAcceptingInvalidSignatures(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			pay := &notificationVerifier{}
			s := &Server{pay: pay}
			path := "/api/v1/cdkshop/payment/notify"
			payload := "out_trade_no=order-1&sign=bad"
			if method == http.MethodGet {
				path += "?" + payload
			}
			req := httptest.NewRequest(method, path, strings.NewReader(payload))
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest || pay.raw != payload {
				t.Fatalf("status=%d payload=%q", w.Code, pay.raw)
			}
		})
	}
}

func TestEmailValidationRejectsQuerySyntaxAndDisplayNames(t *testing.T) {
	for _, email := range []string{"buyer@example.com?subject=test", "Name <buyer@example.com>", "buyer@example.com\nInjected: true"} {
		if validEmail(email) {
			t.Fatalf("accepted invalid order email: %q", email)
		}
	}
	if !validEmail("buyer@example.com") {
		t.Fatal("rejected valid order email")
	}
}
