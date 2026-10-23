package ledger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExactMoney(t *testing.T) {
	for input, expected := range map[string]string{"1": "1.000000", "0.000001": "0.000001", "999999999.999999": "999999999.999999", "12.3456": "12.345600"} {
		actual, err := normalizeAmount(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	for _, input := range []string{"0", "0.000000", "-1", "+1", "1e2", "NaN", "1.1234567", "01", "1000000000", " 1", "1.", ".1"} {
		_, err := normalizeAmount(input)
		require.Error(t, err, input)
	}
}
func TestValidationBoundaries(t *testing.T) {
	emptyRecipients := Settings{ReminderDays: []int{30, 7, 1}}
	require.NoError(t, emptyRecipients.validate())
	require.NotNil(t, emptyRecipients.Recipients)
	entry := EntryInput{Kind: "gift", Amount: "100", Currency: "CREDIT", Category: "送朋友", OccurredOn: "2026-10-23", Note: strings.Repeat("字", 3000)}
	require.NoError(t, entry.validate())
	entry.Kind = "expense"
	require.Error(t, entry.validate())
	entry.Currency = "USD"
	require.NoError(t, entry.validate())
	entry.OccurredOn = "2026-02-30"
	require.Error(t, entry.validate())
	r := ResourceInput{Name: "示例域名", Kind: "domain", Currency: "USD", ExpiresAt: time.Now(), Active: true}
	for _, value := range []string{"https://example.com", " https://example.com/manage "} {
		r.ManagementURL = value
		require.NoError(t, r.validate())
	}
	for _, value := range []string{"javascript:alert(1)", "http://example.com", "https://u:p@example.com", "https://example.com\r\nX: value"} {
		r.ManagementURL = value
		require.Error(t, r.validate(), value)
	}
	cfg := Settings{ReminderDays: []int{1, 30, 7}, Recipients: []string{" Admin@Example.com "}, EmailEnabled: true}
	require.NoError(t, cfg.validate())
	require.Equal(t, []int{30, 7, 1}, cfg.ReminderDays)
	require.Equal(t, "admin@example.com", cfg.Recipients[0])
	cfg.ReminderDays = []int{7, 7}
	require.Error(t, cfg.validate())
	cfg.ReminderDays = []int{7}
	cfg.Recipients = []string{"a@example.com\r\nBcc: b@example.com"}
	require.Error(t, cfg.validate())
}
func TestCalendarThresholdsAndDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, loc)
	for _, test := range []struct {
		expiry time.Time
		want   int
		match  bool
	}{
		{now.Add(-time.Minute), 0, true},
		{time.Date(2026, 3, 8, 10, 0, 0, 0, loc), 1, true},
		{time.Date(2026, 3, 14, 23, 0, 0, 0, loc), 7, true},
		{time.Date(2026, 4, 6, 23, 0, 0, 0, loc), 30, true},
		{time.Date(2026, 4, 7, 0, 0, 0, 0, loc), 366, false},
	} {
		d, ok := thresholdFor(test.expiry, now, []int{30, 7, 1}, loc)
		require.Equal(t, test.match, ok)
		require.Equal(t, test.want, d)
	}
}
func TestDigestEscapingAndCSVFormulas(t *testing.T) {
	text, err := renderDigest([]reminder{{Resource: Resource{Name: "<script>alert(1)</script>", Provider: "A&B", ExpiresAt: time.Now()}, Threshold: 0}}, time.UTC)
	require.NoError(t, err)
	require.NotContains(t, text, "<script>")
	require.Contains(t, text, "&lt;script&gt;")
	require.Contains(t, text, "A&amp;B")
	for _, value := range []string{"=1+2", "+SUM(A1)", " -2", "\t@cmd"} {
		require.Equal(t, "'"+value, csvCell(value))
	}
	require.Equal(t, "普通备注", csvCell("普通备注"))
}
func TestHandlersRejectMissingOrOrdinaryIdentityBeforeDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	for _, role := range []string{"", "user", "admin"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(string(middleware.ContextKeyUserRole), role)
		// An admin role without an authenticated subject must still be rejected.
		h.Entries(c)
		require.Equal(t, http.StatusForbidden, w.Code)
	}
	for _, body := range []string{`{} {}`, `{"unknown":true}`, strings.Repeat("x", 65537)} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		var in Settings
		require.False(t, bind(c, &in))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}
