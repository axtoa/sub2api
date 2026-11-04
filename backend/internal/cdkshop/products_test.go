package cdkshop

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestProductManagementRequiresAdmin(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	userToken, err := SignToken(secret, 17, "user", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{tokenSecret: secret}
	for _, method := range []string{http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodGet} {
		for _, token := range []string{"", "invalid", userToken} {
			t.Run(method+"/"+token, func(t *testing.T) {
				path := "/api/v1/cdkshop/admin/products/product-1"
				if method == http.MethodGet {
					path += "/cdks"
				}
				req := httptest.NewRequest(method, path, strings.NewReader(`{"active":false}`))
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.Handler().ServeHTTP(w, req)
				if w.Code != http.StatusForbidden {
					t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestProductEditValidation(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token, err := SignToken(secret, 1, "admin", time.Now())
	require.NoError(t, err)
	s := &Server{tokenSecret: secret}
	valid := Product{Name: "Plus", PriceCents: 9900, RewardCents: 500, Version: 1}
	for _, edit := range []func(*Product){
		func(p *Product) { p.Name = "   " },
		func(p *Product) { p.Name = strings.Repeat("中", 121) },
		func(p *Product) { p.Spec = strings.Repeat("中", 121) },
		func(p *Product) { p.Description = strings.Repeat("中", 1001) },
		func(p *Product) { p.Instructions = strings.Repeat("中", 1001) },
		func(p *Product) { p.PriceCents = 0 },
		func(p *Product) { p.PriceCents = 10000001 },
		func(p *Product) { p.RewardCents = -1 },
		func(p *Product) { p.RewardCents = 10000001 },
		func(p *Product) { p.Version = 0 },
	} {
		p := valid
		edit(&p)
		body, err := json.Marshal(p)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/cdkshop/admin/products/product-1", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestProductEditSavesReviewedVersionAndRejectsConflicts(t *testing.T) {
	for _, outcome := range []string{"saved", "conflict", "deleted"} {
		t.Run(outcome, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			secret := []byte("0123456789abcdef0123456789abcdef")
			token, err := SignToken(secret, 1, "admin", time.Now())
			require.NoError(t, err)
			s := &Server{db: db, tokenSecret: secret}
			query := `UPDATE cdkshop_products SET name=$2,spec=$3,description=$4,instructions=$5,price_cents=$6,reward_cents=$7,active=$8,version=version+1 WHERE id=$1 AND version=$9 AND deleted_at IS NULL RETURNING version`
			expected := mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("product-1", "New Plus", "month", "description", "redeem here", 12345, 625, true, 3)
			status := http.StatusOK
			if outcome == "saved" {
				expected.WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(4))
			} else {
				expected.WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(SELECT 1 FROM cdkshop_products WHERE id=$1 AND deleted_at IS NULL)`)).WithArgs("product-1").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(outcome == "conflict"))
				status = http.StatusConflict
				if outcome == "deleted" {
					status = http.StatusNotFound
				}
			}
			req := httptest.NewRequest(http.MethodPut, "/api/v1/cdkshop/admin/products/product-1", strings.NewReader(`{"id":"forged","name":" New Plus ","spec":"month","description":"description","instructions":"redeem here","price_cents":12345,"reward_cents":625,"active":true,"version":3}`))
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			require.Equal(t, status, w.Code, w.Body.String())
			if outcome == "saved" {
				require.JSONEq(t, `{"version":4}`, w.Body.String())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProductStatusRequiresExplicitBoolean(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	token, err := SignToken(secret, 1, "admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{tokenSecret: secret}
	for _, body := range []string{`{}`, `{"active":null}`, `{"active":"false"}`, `invalid`} {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/cdkshop/admin/products/product-1", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body = %s, status = %d", body, w.Code)
		}
	}
}
