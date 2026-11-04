//go:build integration

package cdkshop

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type productLifecyclePayment struct{ payment.Provider }

func (p *productLifecyclePayment) CreatePayment(_ context.Context, _ payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	return &payment.CreatePaymentResponse{PayURL: "https://pay.example.com/existing-order"}, nil
}

func (p *productLifecyclePayment) VerifyNotification(_ context.Context, _ string, _ map[string]string) (*payment.PaymentNotification, error) {
	return &payment.PaymentNotification{OrderID: "existing-order", Amount: 99, Status: payment.ProviderStatusSuccess}, nil
}

func TestProductRemovalPreservesPendingDelivery(t *testing.T) {
	dsn := os.Getenv("CDKSHOP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CDKSHOP_TEST_DATABASE_URL is not configured")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("cdkshop_test_%d", time.Now().UnixNano())
	_, err = root.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := root.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, dropErr)
		require.NoError(t, root.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	// Simulate an installed 0.2.15 database before applying the additive schema.
	legacySchema := strings.ReplaceAll(schemaSource, "ALTER TABLE cdkshop_products ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;", "")
	_, err = db.Exec(legacySchema)
	require.NoError(t, err)
	const productID = "22222222-2222-4222-8222-222222222222"
	_, err = db.Exec(`INSERT INTO cdkshop_products(id,name,price_cents,active) VALUES($1,'Plus',9900,true)`, productID)
	require.NoError(t, err)
	for range 2 {
		_, err = db.Exec(schemaSource)
		require.NoError(t, err)
	}
	crypto, err := NewCrypto(strings.Repeat("12", 32))
	require.NoError(t, err)
	encrypted, err := crypto.Encrypt("DELIVER-AFTER-PRODUCT-REMOVAL")
	require.NoError(t, err)
	var cdkID int64
	err = db.QueryRow(`INSERT INTO cdkshop_cdks(product_id,encrypted_code,fingerprint,status,order_id) VALUES($1,$2,'reserved-fingerprint','reserved','existing-order') RETURNING id`, productID, encrypted).Scan(&cdkID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO cdkshop_cdks(product_id,encrypted_code,fingerprint) VALUES($1,$2,'available-fingerprint')`, productID, encrypted)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO cdkshop_orders(id,request_id,user_id,email,recovery_hash,recovery_secret,product_id,product_name,price_cents,reward_cents,cdk_id) VALUES('existing-order','existing-request',17,'buyer@example.com','hash','cipher',$1,'Plus',9900,500,$2)`, productID, cdkID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO cdkshop_settings(key,value) VALUES('store_enabled','true')`)
	require.NoError(t, err)
	secret := []byte("0123456789abcdef0123456789abcdef")
	s := &Server{db: db, crypto: crypto, tokenSecret: secret, pay: &productLifecyclePayment{}, publicURL: "https://shop.example.com"}
	adminToken, err := SignToken(secret, 1, "admin", time.Now())
	require.NoError(t, err)
	userToken, err := SignToken(secret, 17, "user", time.Now())
	require.NoError(t, err)
	call := func(method, path, body, token string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/cdkshop"+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		return w.Body.String()
	}
	path := "/admin/products/" + productID
	// Edit the current product while an older order retains its original snapshot.
	editBody := `{"name":"Plus annual","spec":"12 months","description":"Updated description","instructions":"Updated redemption instructions","price_cents":12345,"reward_cents":625,"active":true,"version":1}`
	require.JSONEq(t, `{"version":2}`, call(http.MethodPut, path, editBody, adminToken, http.StatusOK))
	call(http.MethodPut, path, editBody, adminToken, http.StatusConflict)
	catalog := call(http.MethodGet, "/products", "", "", http.StatusOK)
	require.Contains(t, catalog, `"name":"Plus annual"`)
	require.Contains(t, catalog, `"price_cents":12345`)
	require.Contains(t, catalog, `"instructions":"Updated redemption instructions"`)
	require.NotContains(t, catalog, "DELIVER-AFTER-PRODUCT-REMOVAL")
	orderBefore := call(http.MethodGet, "/orders/existing-order", "", userToken, http.StatusOK)
	require.Contains(t, orderBefore, `"product_name":"Plus"`)
	require.Contains(t, orderBefore, `"price_cents":9900`)
	require.Contains(t, orderBefore, `"reward_cents":500`)
	require.NotContains(t, orderBefore, "DELIVER-AFTER-PRODUCT-REMOVAL")
	call(http.MethodGet, path+"/cdks", "", userToken, http.StatusForbidden)
	stock := call(http.MethodGet, path+"/cdks", "", adminToken, http.StatusOK)
	require.Contains(t, stock, `"code":"DELIVER-AFTER-PRODUCT-REMOVAL"`)
	require.Contains(t, stock, `"total":2`)
	stock = call(http.MethodGet, path+"/cdks?status=reserved", "", adminToken, http.StatusOK)
	require.Contains(t, stock, `"total":1`)
	require.Contains(t, stock, `"order_id":"existing-order"`)
	require.Contains(t, call(http.MethodGet, path+"/cdks?page=2", "", adminToken, http.StatusOK), `"items":[]`)
	call(http.MethodPatch, path, `{"active":false}`, adminToken, http.StatusOK)
	require.JSONEq(t, `{"items":[]}`, call(http.MethodGet, "/products", "", "", http.StatusOK))
	call(http.MethodPatch, path, `{"active":true}`, adminToken, http.StatusOK)
	require.Contains(t, call(http.MethodGet, "/products", "", "", http.StatusOK), productID)
	call(http.MethodDelete, path, "", userToken, http.StatusForbidden)
	call(http.MethodDelete, path, "", adminToken, http.StatusOK)
	require.JSONEq(t, `{"items":[]}`, call(http.MethodGet, "/products", "", "", http.StatusOK))
	require.JSONEq(t, `{"items":[]}`, call(http.MethodGet, "/admin/products", "", adminToken, http.StatusOK))
	call(http.MethodPatch, path, `{"active":true}`, adminToken, http.StatusNotFound)
	call(http.MethodPut, path, editBody, adminToken, http.StatusNotFound)
	call(http.MethodDelete, path, "", adminToken, http.StatusNotFound)
	call(http.MethodPost, path+"/cdks", `{"codes":["NEW-CODE"]}`, adminToken, http.StatusNotFound)
	newOrder := fmt.Sprintf(`{"product_id":%q,"email":"buyer@example.com","request_id":"11111111-1111-4111-8111-111111111111"}`, productID)
	call(http.MethodPost, "/orders", newOrder, userToken, http.StatusNotFound)

	var archived, inactive bool
	err = db.QueryRow(`SELECT deleted_at IS NOT NULL,NOT active FROM cdkshop_products WHERE id=$1`, productID).Scan(&archived, &inactive)
	require.NoError(t, err)
	require.True(t, archived)
	require.True(t, inactive)
	var stockCount, orderCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM cdkshop_cdks WHERE product_id=$1`, productID).Scan(&stockCount)
	require.NoError(t, err)
	require.Equal(t, 2, stockCount)
	err = db.QueryRow(`SELECT COUNT(*) FROM cdkshop_orders WHERE product_id=$1`, productID).Scan(&orderCount)
	require.NoError(t, err)
	require.Equal(t, 1, orderCount)

	// Deleting the product must not prevent an existing buyer paying or receiving it.
	call(http.MethodPost, "/orders/existing-order/payment", "", userToken, http.StatusOK)
	call(http.MethodPost, "/payment/notify", "mock-signed-notification", "", http.StatusOK)
	result := call(http.MethodGet, "/orders/existing-order", "", userToken, http.StatusOK)
	require.Contains(t, result, `"cdk":"DELIVER-AFTER-PRODUCT-REMOVAL"`)
	require.Contains(t, result, `"payment_status":"paid"`)
	require.Contains(t, call(http.MethodGet, path+"/cdks?status=delivered", "", adminToken, http.StatusOK), `"total":1`)
	var rewardCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM cdkshop_reward_tasks WHERE order_id='existing-order'`).Scan(&rewardCount)
	require.NoError(t, err)
	require.Equal(t, 1, rewardCount)
}
