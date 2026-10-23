package upstreambalance

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ Engine *Engine }

func (h *Handler) Stop() {
	if h != nil && h.Engine != nil {
		h.Engine.Stop()
	}
}
func actor(c *gin.Context) (int64, bool) {
	a, ok := middleware.GetAuthSubjectFromContext(c)
	r, yes := middleware.GetUserRoleFromContext(c)
	if !ok || !yes || a.UserID <= 0 || r != "admin" {
		response.Forbidden(c, "需要管理员权限")
		return 0, false
	}
	c.Header("Cache-Control", "no-store")
	return a.UserID, true
}
func failed(c *gin.Context, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, sql.ErrNoRows) {
		response.NotFound(c, "钱包不存在")
	} else if apperrors.Code(e) < 500 {
		response.ErrorFrom(c, e)
	} else {
		response.InternalError(c, "上游余额暂不可用，请稍后重试")
	}
	return true
}
func bind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		response.BadRequest(c, "请求格式无效")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		response.BadRequest(c, "请求格式无效")
		return false
	}
	return true
}
func id(c *gin.Context, optional bool) bool {
	if optional && c.Param("id") == "" {
		return true
	}
	if _, e := uuid.Parse(c.Param("id")); e != nil {
		response.BadRequest(c, "钱包编号无效")
		return false
	}
	return true
}
func (h *Handler) List(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	now := time.Now()
	items, e := h.Engine.List(c.Request.Context(), now)
	if !failed(c, e) {
		response.Success(c, gin.H{"items": items, "server_time": now, "timezone": h.Engine.Store.Location.String()})
	}
}
func (h *Handler) Options(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	items, e := h.Engine.Store.Options(c.Request.Context())
	if !failed(c, e) {
		response.Success(c, gin.H{"items": items})
	}
}
func (h *Handler) Save(c *gin.Context) {
	a, ok := actor(c)
	if !ok || !id(c, true) {
		return
	}
	var in Input
	if !bind(c, &in) {
		return
	}
	w, e := h.Engine.Store.Save(c.Request.Context(), c.Param("id"), in, a)
	if !failed(c, e) {
		response.Success(c, w)
	}
}
func (h *Handler) Calibrate(c *gin.Context) {
	a, ok := actor(c)
	if !ok || !id(c, false) {
		return
	}
	var in Calibration
	if !bind(c, &in) {
		return
	}
	w, e := h.Engine.Store.Calibrate(c.Request.Context(), c.Param("id"), in, a)
	if !failed(c, e) {
		response.Success(c, w)
	}
}
func (h *Handler) Query(c *gin.Context) {
	if _, ok := actor(c); !ok || !id(c, false) {
		return
	}
	w, e := h.Engine.Query(c.Request.Context(), c.Param("id"), true)
	if !failed(c, e) {
		response.Success(c, w)
	}
}
func (h *Handler) History(c *gin.Context) {
	if _, ok := actor(c); !ok || !id(c, false) {
		return
	}
	items, e := h.Engine.Store.History(c.Request.Context(), c.Param("id"))
	if !failed(c, e) {
		response.Success(c, gin.H{"items": items})
	}
}
func (h *Handler) Settings(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	v, e := h.Engine.Store.Settings(c.Request.Context())
	if failed(c, e) {
		return
	}
	deliveries, e := h.Engine.Store.Deliveries(c.Request.Context())
	if !failed(c, e) {
		response.Success(c, gin.H{"settings": v, "deliveries": deliveries, "timezone": h.Engine.Store.Location.String()})
	}
}
func (h *Handler) SaveSettings(c *gin.Context) {
	if _, ok := actor(c); !ok {
		return
	}
	var v Settings
	if !bind(c, &v) {
		return
	}
	v, e := h.Engine.Store.SaveSettings(c.Request.Context(), v)
	if !failed(c, e) {
		response.Success(c, v)
	}
}
