package userinsights

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
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
	now := time.Now()
	f.From, err = parseDate(c.Query("from"), time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
	if err != nil {
		response.BadRequest(c, "Invalid from date")
		return
	}
	f.To, err = parseDate(c.Query("to"), now)
	if err != nil {
		response.BadRequest(c, "Invalid to date")
		return
	}
	f.TodayFrom, err = parseDate(c.Query("today_from"), time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
	if err != nil {
		response.BadRequest(c, "Invalid today_from date")
		return
	}
	f.TodayTo = now
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

func parseDate(raw string, fallback time.Time) (time.Time, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return value, nil
}

func (h *Handler) Ignored(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := h.Store.Ignored(ctx)
	if err != nil {
		response.InternalError(c, "User insights unavailable")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) Candidates(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	if len([]rune(search)) > 100 {
		response.BadRequest(c, "Invalid search")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := h.Store.Candidates(ctx, CandidateFilter{Search: search, Limit: 100})
	if err != nil {
		response.InternalError(c, "User insights unavailable")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) SaveIgnored(c *gin.Context) {
	var request struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid user IDs")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Store.SaveIgnored(ctx, request.UserIDs); err != nil {
		if strings.Contains(err.Error(), "invalid user id") || strings.Contains(err.Error(), "unknown user id") || strings.Contains(err.Error(), "too many") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "User insights unavailable")
		return
	}
	response.Success(c, http.StatusText(http.StatusOK))
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
