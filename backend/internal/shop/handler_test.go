package shop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandlersRequireAuthenticatedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, nil)
	for _, tc := range []struct {
		method, path string
		handler      gin.HandlerFunc
	}{
		{"POST", "/orders", h.Create}, {"GET", "/orders", h.Orders}, {"GET", "/orders/:id", h.Order}, {"POST", "/orders/:id/actions", h.Action},
	} {
		r := gin.New()
		r.Handle(tc.method, tc.path, tc.handler)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, strings.ReplaceAll(tc.path, ":id", "other"), strings.NewReader(`{}`)))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestUserCannotGrantAdminAccessThroughQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, mock := testStore(t)
	h := &Handler{Store: s}
	mock.ExpectQuery(`SELECT payload,version FROM shop_orders WHERE id=$1 AND ($2 OR user_id=$3)`).WithArgs("other", false, int64(42)).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})
	r.GET("/api/v1/shop/orders/:id", h.Order)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/shop/orders/other?admin=true&user_id=1", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateIgnoresForgedApplicantPriceAndPayment(t *testing.T) {
	s, mock := testStore(t)
	h := &Handler{Store: s}
	req := createFixture()
	expectCreateAccount(mock, req, nil, "")
	expectShopEnabled(mock, true)
	expectProduct(mock, t, productFixture())
	mock.ExpectQuery(duplicateSQL).WithArgs(int64(42), req.ProductID, "member@example.com").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(insertOrderSQL).WithArgs(sqlmock.AnyArg(), int64(42), req.ProductID, req.RequestKey, sqlmock.AnyArg(), orderArgument{t, "member@example.com"}, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})
	r.POST("/api/v1/shop/orders", h.Create)
	body := `{"product_id":"` + req.ProductID + `","product_version":3,"accepted":true,"request_key":"` + req.RequestKey + `","user_id":1,"applicant_email":"forged@example.com","amount_cents":1,"payment_status":"PAID"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/shop/orders", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "forged@example.com")
}
