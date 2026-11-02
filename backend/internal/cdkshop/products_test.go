package cdkshop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProductManagementRequiresAdmin(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	userToken, err := SignToken(secret, 17, "user", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{tokenSecret: secret}
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		for _, token := range []string{"", "invalid", userToken} {
			t.Run(method+"/"+token, func(t *testing.T) {
				req := httptest.NewRequest(method, "/api/v1/cdkshop/admin/products/product-1", strings.NewReader(`{"active":false}`))
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
