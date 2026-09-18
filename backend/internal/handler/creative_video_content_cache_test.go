package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreativeVideoCacheRoundTripSupportsRangeRequests(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	task := &service.CreativeVideoTask{TaskID: "vidtask_cache", ContentType: stringPtrForCreativeVideoTest("video/mp4")}

	file, path, err := prepareCreativeVideoCache(task.TaskID)
	require.NoError(t, err)
	_, err = io.WriteString(file, "0123456789")
	require.NoError(t, err)
	require.NoError(t, commitCreativeVideoCache(file, path))

	cached, contentType, found, err := openCreativeVideoCache(task)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "video/mp4", contentType)
	defer func() { _ = cached.Close() }()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/vid/content", nil)
	ctx.Request.Header.Set("Range", "bytes=2-5")
	serveCreativeVideoCache(ctx, task, cached, contentType)

	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "bytes 2-5/10", recorder.Header().Get("Content-Range"))
	body, err := io.ReadAll(recorder.Body)
	require.NoError(t, err)
	require.Equal(t, "2345", string(body))
}

func TestCreativeVideoCacheRejectsPathTraversal(t *testing.T) {
	_, err := creativeVideoCachePath("../outside")
	require.Error(t, err)
}

func stringPtrForCreativeVideoTest(value string) *string {
	return &value
}
