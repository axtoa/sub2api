package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/shop"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type shopAuthUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *shopAuthUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}

func (r *shopAuthUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func TestShopAndExistingPanelRoutesUseRealAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.JWT.Secret = "release-shop-route-test-secret-only"
	cfg.JWT.AccessTokenExpireMinutes = 60
	user := &service.User{ID: 42, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 1}
	repo := &shopAuthUserRepo{user: user}
	auth := service.NewAuthService(nil, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	users := service.NewUserService(repo, nil, nil, nil)
	userToken, err := auth.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	router := gin.New()
	handlers := &handler.Handlers{Shop: shop.NewHandler(nil, nil), Admin: &handler.AdminHandlers{}}
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterUserRoutes(router.Group("/api/v1"), handlers, middleware.NewJWTAuthMiddleware(auth, users, nil, nil), audit, nil, nil)
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, middleware.NewAdminAuthMiddleware(auth, users, nil, nil), audit, stepUp, nil, nil)

	adminRoutes := []struct{ method, path string }{
		{"GET", "/api/v1/admin/upstream-balances"}, {"POST", "/api/v1/admin/upstream-balances"},
		{"GET", "/api/v1/admin/upstream-balances/accounts"}, {"PUT", "/api/v1/admin/upstream-balances/wallet"},
		{"POST", "/api/v1/admin/upstream-balances/wallet/query"}, {"POST", "/api/v1/admin/upstream-balances/wallet/calibrate"},
		{"GET", "/api/v1/admin/upstream-balances/wallet/history"}, {"GET", "/api/v1/admin/upstream-balances/settings"}, {"PUT", "/api/v1/admin/upstream-balances/settings"},
		{"GET", "/api/v1/admin/ledger/entries"}, {"GET", "/api/v1/admin/ledger/export"},
		{"POST", "/api/v1/admin/ledger/entries"}, {"PUT", "/api/v1/admin/ledger/entries/entry"}, {"POST", "/api/v1/admin/ledger/entries/entry/void"},
		{"GET", "/api/v1/admin/ledger/resources"}, {"POST", "/api/v1/admin/ledger/resources"}, {"PUT", "/api/v1/admin/ledger/resources/resource"}, {"POST", "/api/v1/admin/ledger/resources/resource/renew"},
		{"GET", "/api/v1/admin/ledger/history/entry"}, {"GET", "/api/v1/admin/ledger/settings"}, {"PUT", "/api/v1/admin/ledger/settings"},
		{"POST", "/api/v1/admin/quality-monitor/sync"}, {"PUT", "/api/v1/admin/quality-monitor/accounts/1"},
		{"GET", "/api/v1/admin/quality-monitor/snapshot"}, {"PUT", "/api/v1/admin/quality-monitor/settings"},
		{"GET", "/api/v1/admin/quality-monitor/accounts"}, {"POST", "/api/v1/admin/quality-monitor/targets"},
		{"PUT", "/api/v1/admin/quality-monitor/targets/target"}, {"GET", "/api/v1/admin/quality-monitor/targets/target/history"},
		{"POST", "/api/v1/admin/quality-monitor/runs"}, {"GET", "/api/v1/admin/quality-monitor/runs/run"},
		{"GET", "/api/v1/admin/shop/settings"}, {"PUT", "/api/v1/admin/shop/settings"},
		{"GET", "/api/v1/admin/shop/settings/homepage"}, {"PUT", "/api/v1/admin/shop/settings/homepage"},
		{"GET", "/api/v1/admin/shop/settings/homepage/content"}, {"PUT", "/api/v1/admin/shop/settings/homepage/content"},
		{"GET", "/api/v1/admin/shop/settings/external"}, {"PUT", "/api/v1/admin/shop/settings/external"},
		{"GET", "/api/v1/admin/shop/settings/labels"}, {"PUT", "/api/v1/admin/shop/settings/labels/official"}, {"PUT", "/api/v1/admin/shop/settings/labels/external"},
		{"GET", "/api/v1/admin/shop/products"}, {"POST", "/api/v1/admin/shop/products"}, {"PUT", "/api/v1/admin/shop/products/product"},
		{"GET", "/api/v1/admin/shop/orders"}, {"GET", "/api/v1/admin/shop/orders/order"}, {"POST", "/api/v1/admin/shop/orders/order/actions"},
	}
	for _, route := range adminRoutes {
		for _, credentials := range []struct {
			name, token string
			status      int
		}{
			{"anonymous", "", http.StatusUnauthorized},
			{"invalid token", "not-a-valid-token", http.StatusUnauthorized},
			{"ordinary user", userToken, http.StatusForbidden},
		} {
			t.Run(route.method+route.path+"/"+credentials.name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path+"?admin=true", strings.NewReader(`{}`))
				if credentials.token != "" {
					req.Header.Set("Authorization", "Bearer "+credentials.token)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				require.Equal(t, credentials.status, w.Code)
			})
		}
	}

	for _, route := range []struct{ method, path string }{
		{"POST", "/api/v1/shop/orders"}, {"GET", "/api/v1/shop/orders"},
		{"GET", "/api/v1/shop/orders/order"}, {"POST", "/api/v1/shop/orders/order/actions"},
		{"GET", "/api/v1/user/profile"}, {"GET", "/api/v1/keys"}, {"GET", "/api/v1/usage"}, {"POST", "/api/v1/redeem"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}
