package upstreambalance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type isolatedAccountSource struct{ panicOnRead bool }

func (s isolatedAccountSource) GetByID(context.Context, int64) (*service.Account, error) {
	if s.panicOnRead {
		panic("private account credential")
	}
	return exampleAccount(), nil
}

func TestOptionalFeatureInitializationDoesNotFailApplication(t *testing.T) {
	e, err := NewIsolatedEngine(nil, nil, nil)
	require.Nil(t, e)
	require.Error(t, err)
	require.NotPanics(t, func() { (&Handler{}).Stop() })
	require.NotPanics(t, func() { NewEngine(nil, nil, nil, nil).Stop() })
}

func TestTaskPanicsAreContainedAndNextTaskCanRun(t *testing.T) {
	err := runBalanceTask(func() error { panic("private SMTP settings") })
	require.EqualError(t, err, "upstream wallet task unavailable")
	require.NoError(t, runBalanceTask(func() error { return nil }))
	want := errors.New("database unavailable")
	require.ErrorIs(t, runBalanceTask(func() error { return want }), want)
}

func TestProbePanicsBecomeSanitizedWalletFailures(t *testing.T) {
	w := Wallet{Config: exampleConfig()}
	for _, panicOnAccount := range []bool{false, true} {
		e := NewEngine(nil, isolatedAccountSource{panicOnAccount}, nil, nil)
		e.Probe.Send = func(*http.Request, string) (*http.Response, error) { panic("private proxy URL") }
		o, _, reason := e.observe(context.Background(), w)
		require.Nil(t, o)
		require.Equal(t, "internal_error", reason)
	}
}

func TestBalanceFailureCannotBreakOtherHTTPRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, h := range []*Handler{nil, {}, {Engine: NewEngine(nil, nil, nil, nil)}} {
		r := gin.New() // No global Recovery: the optional feature owns recovery.
		r.GET("/balance", h.Isolation, func(c *gin.Context) {
			deadline, ok := c.Request.Context().Deadline()
			require.True(t, ok)
			require.LessOrEqual(t, time.Until(deadline), 25*time.Second)
			panic("private upstream token")
		})
		r.GET("/ledger", func(c *gin.Context) { c.String(http.StatusOK, "ledger available") })
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			require.NotPanics(t, func() { r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/balance", nil)) })
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.NotContains(t, w.Body.String(), "private")
			w = httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ledger", nil))
			require.Equal(t, http.StatusOK, w.Code)
		}
	}
}
