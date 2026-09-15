package handler

import (
	"net/http"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *OpenAIGatewayHandler) CreativeVideoTasks(c *gin.Context) {
	owner, ok := batchImageOwnerFromContext(c)
	if !ok {
		batchImageError(c, infraerrors.New(http.StatusUnauthorized, "API_KEY_REQUIRED", "API key is required"))
		return
	}
	if h == nil || h.creativeVideoService == nil {
		c.JSON(http.StatusOK, service.CreativeVideoTasksResponse{
			Object:            "list",
			Data:              []service.CreativeVideoTaskPublic{},
			RetentionDays:     service.CreativeWorkbenchRetentionDaysDefault,
			MaxRecordsPerUser: service.CreativeWorkbenchMaxRecordsDefault,
			MaxRunningPerUser: service.CreativeWorkbenchVideoRunningDefault,
		})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	got, err := h.creativeVideoService.List(c.Request.Context(), owner, service.CreativeVideoTasksQuery{
		Status: c.Query("status"),
		Limit:  limit,
		Cursor: c.Query("cursor"),
	})
	if err != nil {
		batchImageError(c, err)
		return
	}
	c.JSON(http.StatusOK, got)
}

func (h *OpenAIGatewayHandler) DeleteCreativeVideoTask(c *gin.Context) {
	owner, ok := batchImageOwnerFromContext(c)
	if !ok {
		batchImageError(c, infraerrors.New(http.StatusUnauthorized, "API_KEY_REQUIRED", "API key is required"))
		return
	}
	if h == nil || h.creativeVideoService == nil {
		c.Status(http.StatusNoContent)
		return
	}
	if err := h.creativeVideoService.DeleteRecord(c.Request.Context(), owner, c.Param("request_id")); err != nil {
		batchImageError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
