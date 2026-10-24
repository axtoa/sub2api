package admin

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type consumePriorityAdminStub struct {
	service.AdminService
	account service.Account
	saveErr error
	getErr  error
	updates map[string]any
}

func (s *consumePriorityAdminStub) GetAccount(context.Context, int64) (*service.Account, error) {
	return &s.account, s.getErr
}

func (s *consumePriorityAdminStub) UpdateAccountExtra(_ context.Context, _ int64, updates map[string]any) error {
	s.updates = updates
	return s.saveErr
}

func TestConsumePriorityHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, id, body, platform string
		status                   int
		enabled                  bool
		saveErr, getErr          error
	}{
		{name: "enable", id: "1", body: `{"enabled":true,"priority":-100}`, platform: service.PlatformOpenAI, status: 200, enabled: true},
		{name: "disable", id: "1", body: `{"enabled":false}`, platform: service.PlatformAnthropic, status: 200},
		{name: "unsupported", id: "1", body: `{"enabled":true}`, platform: service.PlatformGemini, status: 400},
		{name: "disable unsupported", id: "1", body: `{"enabled":false}`, platform: service.PlatformGemini, status: 200},
		{name: "missing", id: "1", body: `{}`, status: 400},
		{name: "null", id: "1", body: `{"enabled":null}`, status: 400},
		{name: "string", id: "1", body: `{"enabled":"true"}`, status: 400},
		{name: "bad id", id: "abc", body: `{"enabled":true}`, status: 400},
		{name: "zero id", id: "0", body: `{"enabled":true}`, status: 400},
		{name: "not found", id: "1", body: `{"enabled":true}`, status: 404, getErr: service.ErrAccountNotFound},
		{name: "save failure", id: "1", body: `{"enabled":true}`, platform: service.PlatformOpenAI, status: 500, saveErr: errors.New("save failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &consumePriorityAdminStub{account: service.Account{ID: 1, Platform: tc.platform, Priority: 9}, saveErr: tc.saveErr, getErr: tc.getErr}
			h := &AccountHandler{adminService: s}
			r := gin.New()
			r.POST("/accounts/:id/consume-priority", h.SetConsumePriority)
			req := httptest.NewRequest("POST", "/accounts/"+tc.id+"/consume-priority", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == 200 {
				require.Equal(t, map[string]any{service.ConsumePriorityExtraKey: tc.enabled}, s.updates)
			} else if tc.saveErr == nil {
				require.Nil(t, s.updates)
			}
			require.Equal(t, 9, s.account.Priority)
		})
	}
}
