package userinsights

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ Store *Store }

func NewHandler(db *sql.DB) *Handler { return &Handler{Store: NewStore(db)} }

func (h *Handler) List(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.BadRequest(c, "Invalid page")
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		response.BadRequest(c, "Invalid page_size")
		return
	}
	f := Filter{
		Page: page, Size: size, Search: strings.TrimSpace(c.Query("search")), Segment: c.DefaultQuery("segment", "all"),
		SortBy: c.DefaultQuery("sort_by", "redeemed_amount"), SortOrder: strings.ToLower(c.DefaultQuery("sort_order", "desc")),
	}
	if err := f.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := h.Store.List(ctx, f)
	if err != nil {
		response.InternalError(c, "User insights unavailable")
		return
	}
	response.Success(c, result)
}

func (h *Handler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := h.Store.Detail(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "User not found")
		return
	}
	if err != nil {
		response.InternalError(c, "User insights unavailable")
		return
	}
	response.Success(c, result)
}
