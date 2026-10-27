package shop

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type settingsWriter interface {
	SetOfficialShopEnabled(context.Context, bool) error
	GetExternalShopSettings(context.Context) (service.ExternalShopSettings, error)
	SetExternalShopSettings(context.Context, bool, string) error
	GetShopLabels(context.Context) (service.ShopLabels, error)
	SetShopLabel(context.Context, string, string) (string, error)
}

type Handler struct {
	Store    *Store
	settings settingsWriter
}

func NewHandler(db *sql.DB, settings settingsWriter) *Handler {
	return &Handler{Store: &Store{DB: db}, settings: settings}
}

func (h *Handler) Availability(c *gin.Context) {
	enabled, err := h.Store.Enabled(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"enabled": enabled})
	}
}

func (h *Handler) SetAvailability(c *gin.Context) {
	if role, ok := middleware.GetUserRoleFromContext(c); !ok || role != "admin" {
		response.Forbidden(c, "需要管理员权限")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "请提供有效的启用状态")
		return
	}
	if !response.ErrorFrom(c, h.settings.SetOfficialShopEnabled(c.Request.Context(), *req.Enabled)) {
		response.Success(c, gin.H{"enabled": *req.Enabled})
	}
}

func (h *Handler) ExternalSettings(c *gin.Context) {
	settings, err := h.settings.GetExternalShopSettings(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, settings)
	}
}

func (h *Handler) SetExternalSettings(c *gin.Context) {
	if role, ok := middleware.GetUserRoleFromContext(c); !ok || role != "admin" {
		response.Forbidden(c, "需要管理员权限")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		Enabled *bool  `json:"enabled"`
		URL     string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil || (req.URL != "" && !service.IsValidExternalPurchaseURL(req.URL)) || (*req.Enabled && req.URL == "") {
		response.BadRequest(c, "请提供有效的 HTTPS 代购商城链接和启用状态")
		return
	}
	if !response.ErrorFrom(c, h.settings.SetExternalShopSettings(c.Request.Context(), *req.Enabled, req.URL)) {
		response.Success(c, service.ExternalShopSettings{Enabled: *req.Enabled, URL: req.URL})
	}
}

func (h *Handler) Labels(c *gin.Context) {
	labels, err := h.settings.GetShopLabels(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, labels)
	}
}

func (h *Handler) SetLabel(c *gin.Context) {
	if role, ok := middleware.GetUserRoleFromContext(c); !ok || role != "admin" {
		response.Forbidden(c, "需要管理员权限")
		return
	}
	channel := c.Param("channel")
	if channel != "official" && channel != "external" {
		response.BadRequest(c, "代购渠道无效")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var req struct {
		Name *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil || !service.IsValidShopLabel(*req.Name) {
		response.BadRequest(c, "名称不能超过 24 个字符或包含控制字符")
		return
	}
	name, err := h.settings.SetShopLabel(c.Request.Context(), channel, *req.Name)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"name": name})
	}
}

func (h *Handler) Products(c *gin.Context) {
	admin := strings.HasPrefix(c.FullPath(), "/api/v1/admin/")
	if !admin {
		enabled, err := h.Store.Enabled(c.Request.Context())
		if response.ErrorFrom(c, err) {
			return
		}
		if !enabled {
			response.ErrorFrom(c, shopDisabled())
			return
		}
	}
	items, err := h.Store.Products(c.Request.Context(), admin)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) SaveProduct(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var p Product
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "商品信息格式错误")
		return
	}
	if c.Param("id") == "" {
		p.ID, p.Version = "", 0
	} else {
		p.ID = c.Param("id")
	}
	item, err := h.Store.SaveProduct(c.Request.Context(), p)
	if !response.ErrorFrom(c, err) {
		response.Success(c, item)
	}
}

func (h *Handler) Create(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req CreateOrder
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "订单信息格式错误")
		return
	}
	order, err := h.Store.Create(c.Request.Context(), subject.UserID, req)
	if !response.ErrorFrom(c, err) {
		response.Success(c, order)
	}
}

func (h *Handler) Orders(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		response.BadRequest(c, "页码超出范围")
		return
	}
	q := strings.TrimSpace(c.Query("q"))
	if len(q) > 200 {
		response.BadRequest(c, "搜索内容过长")
		return
	}
	items, total, err := h.Store.Orders(c.Request.Context(), OrderFilter{
		UserID: subject.UserID, Admin: strings.HasPrefix(c.FullPath(), "/api/v1/admin/"),
		Query: q, Payment: c.Query("payment"), Fulfillment: c.Query("fulfillment"), Product: c.Query("product"),
		From: c.Query("from"), To: c.Query("to"), Page: page, PageSize: 20,
	})
	if !response.ErrorFrom(c, err) {
		response.Paginated(c, items, total, page, 20)
	}
}

func (h *Handler) Order(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	admin := strings.HasPrefix(c.FullPath(), "/api/v1/admin/")
	order, err := h.Store.Get(c.Request.Context(), c.Param("id"), subject.UserID, admin)
	if !response.ErrorFrom(c, err) {
		response.Success(c, order)
	}
}

func (h *Handler) Action(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var action OrderAction
	if err := c.ShouldBindJSON(&action); err != nil {
		response.BadRequest(c, "操作信息格式错误")
		return
	}
	admin := strings.HasPrefix(c.FullPath(), "/api/v1/admin/")
	order, err := h.Store.Act(c.Request.Context(), c.Param("id"), subject.UserID, admin, action)
	if !response.ErrorFrom(c, err) {
		response.Success(c, order)
	}
}
