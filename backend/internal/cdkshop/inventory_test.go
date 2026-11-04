package cdkshop

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestInventoryListsDecryptedCodesWithBoundedPagination(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		crypto, err := NewCrypto(strings.Repeat("12", 32))
		require.NoError(t, err)
		encrypted, err := crypto.Encrypt("ADMIN-CODE-ONLY")
		require.NoError(t, err)
		if corrupt {
			encrypted = "invalid-cipher"
		}
		secret := []byte("0123456789abcdef0123456789abcdef")
		token, err := SignToken(secret, 1, "admin", time.Now())
		require.NoError(t, err)
		s := &Server{db: db, crypto: crypto, tokenSecret: secret}
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(SELECT 1 FROM cdkshop_products WHERE id=$1)`)).WithArgs("product-1").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM cdkshop_cdks WHERE product_id=$1 AND ($2='' OR status=$2)`)).WithArgs("product-1", "reserved").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(51))
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id,encrypted_code,status,COALESCE(order_id,''),created_at FROM cdkshop_cdks WHERE product_id=$1 AND ($2='' OR status=$2) ORDER BY id DESC LIMIT $3 OFFSET $4`)).WithArgs("product-1", "reserved", 50, 50).WillReturnRows(sqlmock.NewRows([]string{"id", "encrypted_code", "status", "order_id", "created_at"}).AddRow(12, encrypted, "reserved", "order-1", time.Now()))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cdkshop/admin/products/product-1/cdks?status=reserved&page=2", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		if corrupt {
			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Body.String(), `"code_error":"inventory_decrypt_failed"`)
			require.NotContains(t, w.Body.String(), encrypted)
		} else {
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"code":"ADMIN-CODE-ONLY"`)
			require.Contains(t, w.Body.String(), `"order_id":"order-1"`)
			require.Contains(t, w.Body.String(), `"total":51`)
			require.NotContains(t, w.Body.String(), encrypted)
		}
		mock.ExpectClose()
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestInventoryRejectsInvalidFiltersBeforeQuerying(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token, err := SignToken(secret, 1, "admin", time.Now())
	require.NoError(t, err)
	s := &Server{tokenSecret: secret}
	for _, query := range []string{"page=0", "page=-1", "page=1000001", "page=invalid", "status=unknown"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cdkshop/admin/products/product-1/cdks?"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
}
