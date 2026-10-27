package shop

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreateFailsClosedWhenShopIsDisabledOrSettingsCannotBeRead(t *testing.T) {
	for _, dbFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "database failure"}[dbFailure], func(t *testing.T) {
			s, mock := testStore(t)
			req := createFixture()
			expectCreateAccount(mock, req, nil, "")
			if dbFailure {
				mock.ExpectQuery(enabledForShareSQL).WithArgs(service.SettingKeyOfficialShopEnabled).WillReturnError(errors.New("database unavailable"))
			} else {
				expectShopEnabled(mock, false)
			}
			mock.ExpectRollback()
			order, err := s.Create(context.Background(), 42, req)
			require.Error(t, err)
			require.Nil(t, order)
			if !dbFailure {
				require.Equal(t, http.StatusForbidden, appErrors.Code(err))
				require.Equal(t, "SHOP_DISABLED", appErrors.Reason(err))
			}
		})
	}
}

type testSettingsWriter struct {
	values   []bool
	err      error
	external service.ExternalShopSettings
}

func (s *testSettingsWriter) SetOfficialShopEnabled(_ context.Context, enabled bool) error {
	s.values = append(s.values, enabled)
	return s.err
}

func (s *testSettingsWriter) GetExternalShopSettings(context.Context) (service.ExternalShopSettings, error) {
	return s.external, s.err
}

func (s *testSettingsWriter) SetExternalShopSettings(_ context.Context, enabled bool, link string) error {
	if s.err != nil {
		return s.err
	}
	s.external = service.ExternalShopSettings{Enabled: enabled, URL: link}
	return nil
}

func TestAvailabilityUpdateRequiresAdminAndABoolean(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		role, body string
		status     int
	}{
		{"", `{"enabled":false}`, 403}, {"user", `{"enabled":false}`, 403},
		{"admin", `{}`, 400}, {"admin", `{"enabled":null}`, 400}, {"admin", `{"enabled":"false"}`, 400},
		{"admin", `{"enabled":0}`, 400}, {"admin", `{"enabled":false,"x":"` + strings.Repeat("x", 1024) + `"}`, 400},
		{"admin", `{"enabled":false}`, 200}, {"admin", `{"enabled":true}`, 200},
	} {
		t.Run(tc.role+tc.body[:min(len(tc.body), 30)], func(t *testing.T) {
			writer := &testSettingsWriter{}
			h := NewHandler(nil, writer)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if tc.role != "" {
					c.Set(string(middleware.ContextKeyUserRole), tc.role)
				}
				c.Next()
			})
			r.PUT("/api/v1/admin/shop/settings", h.SetAvailability)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings", strings.NewReader(tc.body)))
			require.Equal(t, tc.status, w.Code)
			if tc.status == 200 {
				require.Len(t, writer.values, 1)
			} else {
				require.Empty(t, writer.values)
			}
		})
	}
}

func TestAvailabilityUpdateReportsPersistenceFailures(t *testing.T) {
	writer := &testSettingsWriter{err: errors.New("write failed")}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), "admin"); c.Next() })
	r.PUT("/api/v1/admin/shop/settings", h.SetAvailability)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings", strings.NewReader(`{"enabled":false}`)))
	require.Equal(t, 500, w.Code)
}

func TestExternalShopSettingsUpdateIsIndependent(t *testing.T) {
	writer := &testSettingsWriter{external: service.ExternalShopSettings{URL: "https://wzyp.cn/shop/best-ai"}}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), "admin"); c.Next() })
	r.GET("/api/v1/admin/shop/settings/external", h.ExternalSettings)
	r.PUT("/api/v1/admin/shop/settings/external", h.SetExternalSettings)

	for _, body := range []string{`{"enabled":true,"url":"javascript:alert(1)"}`, `{"enabled":true,"url":""}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/external", strings.NewReader(body)))
		require.Equal(t, 400, w.Code)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/external", strings.NewReader(`{"enabled":true,"url":"https://example.com/shop"}`)))
	require.Equal(t, 200, w.Code)
	require.Empty(t, writer.values)
	require.Equal(t, service.ExternalShopSettings{Enabled: true, URL: "https://example.com/shop"}, writer.external)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/shop/settings/external", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"url":"https://example.com/shop"`)
}

func TestExternalShopSettingsUpdateRequiresAdmin(t *testing.T) {
	writer := &testSettingsWriter{}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), "user"); c.Next() })
	r.PUT("/api/v1/admin/shop/settings/external", h.SetExternalSettings)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/external", strings.NewReader(`{"enabled":true,"url":"https://example.com/shop"}`)))
	require.Equal(t, 403, w.Code)
	require.Equal(t, service.ExternalShopSettings{}, writer.external)
}

func TestCatalogueClosedToUsersButStillAvailableToAdministrators(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(map[bool]string{false: "user", true: "admin"}[admin], func(t *testing.T) {
			s, mock := testStore(t)
			h := &Handler{Store: s}
			path := "/api/v1/shop/products"
			if admin {
				path = "/api/v1/admin/shop/products"
				mock.ExpectQuery(`SELECT payload, version FROM shop_products WHERE $1 OR (payload->>'active')::boolean ORDER BY (payload->>'sort')::integer, id`).WithArgs(true).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}))
			} else {
				mock.ExpectQuery(enabledSQL).WithArgs(service.SettingKeyOfficialShopEnabled).WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))
			}
			r := gin.New()
			r.GET(path, h.Products)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if admin {
				require.Equal(t, 200, w.Code)
			} else {
				require.Equal(t, 403, w.Code)
				require.Contains(t, w.Body.String(), "SHOP_DISABLED")
			}
		})
	}
}

func TestAvailabilityReturnsPersistedState(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s, mock := testStore(t)
		mock.ExpectQuery(enabledSQL).WithArgs(service.SettingKeyOfficialShopEnabled).WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(enabled))
		h := &Handler{Store: s}
		r := gin.New()
		r.GET("/api/v1/admin/shop/settings", h.Availability)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/shop/settings", nil))
		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), `"enabled":`+map[bool]string{true: "true", false: "false"}[enabled])
	}
}
