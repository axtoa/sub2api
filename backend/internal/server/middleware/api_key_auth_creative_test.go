//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthCreativeTaskReads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reads := []struct{ route, path string }{
		{"/v1/models", "/v1/models"},
		{"/models", "/models"},
		{"/v1/images/batches", "/v1/images/batches"},
		{"/v1/images/batches/models", "/v1/images/batches/models"},
		{"/v1/images/batches/:id", "/v1/images/batches/batch_1"},
		{"/v1/images/batches/:id/items", "/v1/images/batches/batch_1/items"},
		{"/v1/images/batches/:id/items/:custom_id/content", "/v1/images/batches/batch_1/items/item_1/content"},
		{"/v1/images/batches/:id/download", "/v1/images/batches/batch_1/download"},
		{"/v1/videos/tasks", "/v1/videos/tasks"},
		{"/v1/videos/studio-tasks", "/v1/videos/studio-tasks"},
		{"/v1/videos/generations/:request_id", "/v1/videos/generations/video_1"},
		{"/v1/videos/generations/:request_id/content", "/v1/videos/generations/video_1/content"},
		{"/v1/videos/edits/:request_id", "/v1/videos/edits/video_1"},
		{"/v1/videos/edits/:request_id/content", "/v1/videos/edits/video_1/content"},
		{"/v1/videos/extensions/:request_id", "/v1/videos/extensions/video_1"},
		{"/v1/videos/extensions/:request_id/content", "/v1/videos/extensions/video_1/content"},
		{"/v1/videos/:request_id", "/v1/videos/video_1"},
		{"/v1/videos/:request_id/content", "/v1/videos/video_1/content"},
	}
	for _, state := range []string{"zero_balance", "quota_exhausted", "expired", "disabled", "inactive_user", "missing_key"} {
		t.Run(state, func(t *testing.T) {
			user := &service.User{ID: 7, Status: service.StatusActive, Role: service.RoleUser, Balance: 0}
			key := &service.APIKey{ID: 100, UserID: user.ID, Key: "creative-read-test", Status: service.StatusActive, User: user}
			wantRead := http.StatusOK
			switch state {
			case "quota_exhausted":
				key.Status = service.StatusAPIKeyQuotaExhausted
				key.Quota, key.QuotaUsed = 1, 1
			case "expired":
				key.Status = service.StatusAPIKeyExpired
				expired := time.Now().Add(-time.Hour)
				key.ExpiresAt = &expired
			case "disabled":
				key.Status = "disabled"
				wantRead = http.StatusUnauthorized
			case "inactive_user":
				user.Status = "disabled"
				wantRead = http.StatusUnauthorized
			case "missing_key":
				wantRead = http.StatusUnauthorized
			}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
			svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
			router := gin.New()
			router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
			ok := func(c *gin.Context) { c.Status(http.StatusOK) }
			for _, read := range reads {
				router.GET(read.route, ok)
			}
			generations := []string{"/v1/images/batches", "/v1/images/generations", "/v1/videos", "/v1/videos/generations", "/v1/videos/edits", "/v1/videos/extensions", "/v1/chat/completions"}
			for _, path := range generations {
				router.POST(path, ok)
			}
			request := func(method, path string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(method, path, nil)
				if state != "missing_key" {
					req.Header.Set("Authorization", "Bearer "+key.Key)
				}
				router.ServeHTTP(w, req)
				return w
			}
			for _, read := range reads {
				require.Equal(t, wantRead, request(http.MethodGet, read.path).Code, read.path)
			}
			for _, path := range generations {
				w := request(http.MethodPost, path)
				require.GreaterOrEqual(t, w.Code, 400, path)
				if state == "zero_balance" {
					requireAPIKeyAuthError(t, w, "INSUFFICIENT_BALANCE", "Insufficient account balance")
				}
			}
		})
	}
}
