package ledger

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ Runner *Runner }

func (h *Handler) Stop() { h.Runner.Stop() }

func actor(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	role, roleOK := middleware.GetUserRoleFromContext(c)
	if !ok || subject.UserID <= 0 || !roleOK || role != "admin" {
		response.Forbidden(c, "需要管理员权限")
		return 0, false
	}
	return subject.UserID, true
}
func failed(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "记录不存在")
	} else if apperrors.Code(err) < 500 {
		response.ErrorFrom(c, err)
	} else {
		response.InternalError(c, "账本暂不可用，请稍后重试")
	}
	return true
}
func bind(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		response.BadRequest(c, "请求格式无效")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		response.BadRequest(c, "请求只能包含一个 JSON 对象")
		return false
	}
	return true
}
func validID(c *gin.Context, optional bool) bool {
	if optional && c.Param("id") == "" {
		return true
	}
	if _, err := uuid.Parse(c.Param("id")); err != nil {
		response.BadRequest(c, "记录编号无效")
		return false
	}
	return true
}
func (h *Handler) filter(c *gin.Context) (Filter, error) {
	page := 1
	if p := c.Query("page"); p != "" {
		var err error
		page, err = strconv.Atoi(p)
		if err != nil {
			return Filter{}, invalid("页码无效")
		}
	}
	f := Filter{Month: c.DefaultQuery("month", time.Now().In(h.Runner.Location).Format("2006-01")), Page: page, Query: c.Query("q"), Kind: c.Query("kind"), Currency: c.Query("currency"), ResourceID: c.Query("resource_id"), Status: c.DefaultQuery("status", "active")}
	return f, f.validate()
}
func (h *Handler) Entries(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	f, err := h.filter(c)
	if failed(c, err) {
		return
	}
	items, err := h.Runner.Store.Entries(c.Request.Context(), f, false)
	items.Today = time.Now().In(h.Runner.Location).Format(time.DateOnly)
	if !failed(c, err) {
		response.Success(c, items)
	}
}

// Neutralize spreadsheet formula interpretation without changing stored values.
func csvCell(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + s
	}
	return s
}
func (h *Handler) Export(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	f, err := h.filter(c)
	if failed(c, err) {
		return
	}
	items, err := h.Runner.Store.Entries(c.Request.Context(), f, true)
	if failed(c, err) {
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="ledger.csv"`)
	c.Header("Cache-Control", "no-store")
	_, _ = c.Writer.Write([]byte("\xef\xbb\xbf"))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"ID", "日期", "类型", "金额", "币种", "分类", "对方/用途", "备注", "关联资源ID", "已作废"})
	for _, e := range items.Items {
		resource := ""
		if e.ResourceID != nil {
			resource = *e.ResourceID
		}
		row := []string{e.ID, e.OccurredOn, e.Kind, e.Amount, e.Currency, e.Category, e.Counterparty, e.Note, resource, strconv.FormatBool(e.Voided)}
		for i := range row {
			row[i] = csvCell(row[i])
		}
		_ = w.Write(row)
	}
	w.Flush()
}
func (h *Handler) SaveEntry(c *gin.Context) {
	a, ok := actor(c)
	if !ok || !validID(c, true) {
		return
	}
	var in EntryInput
	if !bind(c, &in) {
		return
	}
	item, err := h.Runner.Store.SaveEntry(c.Request.Context(), c.Param("id"), in, a)
	if !failed(c, err) {
		response.Success(c, item)
	}
}
func (h *Handler) VoidEntry(c *gin.Context) {
	a, ok := actor(c)
	if !ok || !validID(c, false) {
		return
	}
	var in VersionInput
	if !bind(c, &in) {
		return
	}
	item, err := h.Runner.Store.VoidEntry(c.Request.Context(), c.Param("id"), in, a)
	if !failed(c, err) {
		response.Success(c, item)
	}
}
func (h *Handler) Resources(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	items, err := h.Runner.Store.Resources(c.Request.Context())
	if !failed(c, err) {
		response.Success(c, gin.H{"items": items, "server_time": time.Now(), "timezone": h.Runner.Location.String()})
	}
}
func (h *Handler) SaveResource(c *gin.Context) {
	a, ok := actor(c)
	if !ok || !validID(c, true) {
		return
	}
	var in ResourceInput
	if !bind(c, &in) {
		return
	}
	item, err := h.Runner.Store.SaveResource(c.Request.Context(), c.Param("id"), in, a)
	if !failed(c, err) {
		response.Success(c, item)
	}
}
func (h *Handler) Renew(c *gin.Context) {
	a, ok := actor(c)
	if !ok || !validID(c, false) {
		return
	}
	var in RenewalInput
	if !bind(c, &in) {
		return
	}
	item, err := h.Runner.Store.Renew(c.Request.Context(), c.Param("id"), in, a)
	if !failed(c, err) {
		response.Success(c, item)
	}
}
func (h *Handler) History(c *gin.Context) {
	if _, ok := actor(c); !ok || !validID(c, false) {
		return
	}
	items, err := h.Runner.Store.History(c.Request.Context(), c.Param("id"))
	if !failed(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}
func (h *Handler) Settings(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	cfg, err := h.Runner.Store.Settings(c.Request.Context())
	if failed(c, err) {
		return
	}
	deliveries, err := h.Runner.Store.Deliveries(c.Request.Context())
	if !failed(c, err) {
		response.Success(c, gin.H{"settings": cfg, "deliveries": deliveries, "timezone": h.Runner.Location.String()})
	}
}
func (h *Handler) SaveSettings(c *gin.Context) {
	a, ok := actor(c)
	if !ok {
		return
	}
	var cfg Settings
	if !bind(c, &cfg) {
		return
	}
	saved, err := h.Runner.Store.SaveSettings(c.Request.Context(), cfg, a)
	if !failed(c, err) {
		response.Success(c, saved)
	}
}
