package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SetConsumePriority changes only the independent preference bit. UpdateExtra
// atomically merges this key and publishes the existing scheduler outbox event.
func (h *AccountHandler) SetConsumePriority(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "A boolean enabled field is required")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if *req.Enabled && !account.SupportsConsumePriority() {
		response.BadRequest(c, "Consume priority supports OpenAI and Anthropic accounts")
		return
	}
	if err := h.adminService.UpdateAccountExtra(c.Request.Context(), id, map[string]any{service.ConsumePriorityExtraKey: *req.Enabled}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": *req.Enabled})
}
