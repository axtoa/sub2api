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
	homepage []bool
	content  *service.ShopHomepageContent
	err      error
	external service.ExternalShopSettings
	labels   service.ShopLabels
}

func (s *testSettingsWriter) SetOfficialShopEnabled(_ context.Context, enabled bool) error {
	s.values = append(s.values, enabled)
	return s.err
}

func (s *testSettingsWriter) GetShopHomepageEnabled(context.Context) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if len(s.homepage) == 0 {
		return false, nil
	}
	return s.homepage[len(s.homepage)-1], nil
}

func (s *testSettingsWriter) SetShopHomepageEnabled(_ context.Context, enabled bool) error {
	if s.err != nil {
		return s.err
	}
	s.homepage = append(s.homepage, enabled)
	return nil
}

func (s *testSettingsWriter) GetShopHomepageContent(context.Context) (*service.ShopHomepageContent, error) {
	return s.content, s.err
}

func (s *testSettingsWriter) SetShopHomepageContent(_ context.Context, content service.ShopHomepageContent) error {
	if s.err != nil {
		return s.err
	}
	s.content = &content
	return nil
}

func TestHomepageContentIsValidatedAndIndependent(t *testing.T) {
	writer := &testSettingsWriter{external: service.ExternalShopSettings{Enabled: true, URL: "https://example.com/shop"}}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserRole), c.GetHeader("X-Test-Role"))
		c.Next()
	})
	r.GET("/api/v1/admin/shop/settings/homepage/content", h.HomepageContent)
	r.PUT("/api/v1/admin/shop/settings/homepage/content", h.SetHomepageContent)
	for _, tc := range []struct {
		role, body string
		status     int
	}{
		{"user", `{"title":"代购","lead":"看看","cards":[]}`, 403},
		{"admin", `{"title":"","lead":"看看","cards":[]}`, 400},
		{"admin", `{"title":"代购","lead":"看看","cards":[{"title":"Plus","summary":"简介","bonus_cents":-1}]}`, 400},
		{"admin", `{"title":"代购","lead":"看看","cards":[{"title":"Plus","summary":"简介","price_hint":"约 ¥200/月","bonus_cents":500}]}`, 200},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/homepage/content", strings.NewReader(tc.body))
		req.Header.Set("X-Test-Role", tc.role)
		r.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code)
	}
	require.Equal(t, int64(500), writer.content.Cards[0].BonusCents)
	require.Empty(t, writer.values)
	require.Empty(t, writer.homepage)
	require.True(t, writer.external.Enabled)
}

func TestHomepageSwitchIsIndependentAndAdminOnly(t *testing.T) {
	writer := &testSettingsWriter{external: service.ExternalShopSettings{Enabled: true, URL: "https://example.com/shop"}}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserRole), c.GetHeader("X-Test-Role"))
		c.Next()
	})
	r.GET("/api/v1/admin/shop/settings/homepage", h.HomepageSettings)
	r.PUT("/api/v1/admin/shop/settings/homepage", h.SetHomepageSettings)
	for _, tc := range []struct {
		role, body string
		status     int
	}{
		{"user", `{"enabled":false}`, 403},
		{"admin", `{}`, 400},
		{"admin", `{"enabled":false}`, 200},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/homepage", strings.NewReader(tc.body))
		req.Header.Set("X-Test-Role", tc.role)
		r.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code)
	}
	require.Equal(t, []bool{false}, writer.homepage)
	require.Empty(t, writer.values)
	require.True(t, writer.external.Enabled)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/shop/settings/homepage", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"enabled":false`)
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

func (s *testSettingsWriter) GetShopLabels(context.Context) (service.ShopLabels, error) {
	return s.labels, s.err
}

func (s *testSettingsWriter) SetShopLabel(_ context.Context, channel, name string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if channel == "official" {
		s.labels.OfficialName = name
	} else {
		s.labels.ExternalName = name
	}
	return name, nil
}

func TestShopLabelsUpdateOnlyOneName(t *testing.T) {
	writer := &testSettingsWriter{
		external: service.ExternalShopSettings{Enabled: true, URL: "https://example.com/shop"},
		labels:   service.ShopLabels{OfficialName: "原名称", ExternalName: "外部商城"},
	}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), "admin"); c.Next() })
	r.GET("/api/v1/admin/shop/settings/labels", h.Labels)
	r.PUT("/api/v1/admin/shop/settings/labels/:channel", h.SetLabel)
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"official", `{"name":"新名称"}`, 200},
		{"external", `{"name":""}`, 200},
		{"unknown", `{"name":"错"}`, 400},
		{"official", `{}`, 400},
		{"official", `{"name":"` + strings.Repeat("a", 25) + `"}`, 400},
		{"official", `{"name":"bad\u202e"}`, 400},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/labels/"+tc.path, strings.NewReader(tc.body)))
		require.Equal(t, tc.status, w.Code, tc.path, tc.body)
	}
	require.Equal(t, service.ShopLabels{OfficialName: "新名称", ExternalName: ""}, writer.labels)
	require.Equal(t, service.ExternalShopSettings{Enabled: true, URL: "https://example.com/shop"}, writer.external)
	require.Empty(t, writer.values)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/shop/settings/labels", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"official_name":"新名称"`)
}

func TestShopLabelUpdateRequiresAdmin(t *testing.T) {
	writer := &testSettingsWriter{}
	h := NewHandler(nil, writer)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUserRole), "user"); c.Next() })
	r.PUT("/api/v1/admin/shop/settings/labels/:channel", h.SetLabel)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/admin/shop/settings/labels/official", strings.NewReader(`{"name":"new"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, writer.labels.OfficialName)
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
