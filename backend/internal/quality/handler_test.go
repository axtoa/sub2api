package quality

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mixedPlatformAccounts struct{}

func (mixedPlatformAccounts) GetByID(_ context.Context, id int64) (*service.Account, error) {
	platform := service.PlatformOpenAI
	if id == 2 {
		platform = service.PlatformAnthropic
	}
	return &service.Account{ID: id, Platform: platform, Type: service.AccountTypeAPIKey}, nil
}

func TestQualityMixedPlatformsRejectedBeforeWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"account_ids":[1,2],"models":["text-model"]}`))
	c.Set(string(middleware.ContextKeyUserRole), "admin")
	h := &Handler{Runner: &Runner{Accounts: mixedPlatformAccounts{}}}
	require.NotPanics(t, func() { h.CreateTargets(c) })
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQualityAdminAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{}
	methods := []gin.HandlerFunc{handler.Snapshot, handler.SyncAccounts, handler.SaveAccount, handler.SaveSettings, handler.Accounts, handler.CreateTargets, handler.SaveTarget, handler.Run, handler.RunStatuses, handler.History, handler.Detail}
	for _, role := range []string{"", "user"} {
		for _, method := range methods {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			if role != "" {
				c.Set(string(middleware.ContextKeyUserRole), role)
			}
			require.NotPanics(t, func() { method(c) })
			require.Equal(t, http.StatusForbidden, w.Code)
		}
	}
}

func TestQualityStrictRequestValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"enabled":true,"hidden":1}`, `{"enabled":true} {}`, strings.Repeat("x", 65537)} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		var req struct {
			Enabled bool `json:"enabled"`
		}
		require.False(t, bind(c, &req))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	for _, body := range []string{`{"target_ids":["not-a-uuid"],"kind":"quality"}`, `{"target_ids":[],"kind":"cache"}`, `{"target_ids":["something"],"kind":"execute"}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		c.Set(string(middleware.ContextKeyUserRole), "admin")
		(&Handler{}).Run(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}
