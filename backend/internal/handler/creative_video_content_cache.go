package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const creativeVideoCacheDirectoryName = "creative-video-cache"

func creativeVideoCacheRoot() string {
	if dataDir := strings.TrimSpace(os.Getenv("DATA_DIR")); dataDir != "" {
		return filepath.Join(dataDir, creativeVideoCacheDirectoryName)
	}
	if info, err := os.Stat("/app/data"); err == nil && info.IsDir() {
		return filepath.Join("/app/data", creativeVideoCacheDirectoryName)
	}
	return filepath.Join(".", "data", creativeVideoCacheDirectoryName)
}

func creativeVideoCachePath(taskID string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || filepath.Base(taskID) != taskID ||
		strings.ContainsAny(taskID, `/\`) {
		return "", fmt.Errorf("invalid creative video cache task id")
	}
	return filepath.Join(creativeVideoCacheRoot(), taskID+".video"), nil
}

func openCreativeVideoCache(task *service.CreativeVideoTask) (*os.File, string, bool, error) {
	if task == nil {
		return nil, "", false, nil
	}
	path, err := creativeVideoCachePath(task.TaskID)
	if err != nil {
		return nil, "", false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, "", false, err
	}
	if info.Size() <= 0 {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, "", false, nil
	}
	contentType := "video/mp4"
	if task.ContentType != nil && strings.TrimSpace(*task.ContentType) != "" {
		contentType = strings.TrimSpace(*task.ContentType)
	}
	return file, contentType, true, nil
}

func prepareCreativeVideoCache(taskID string) (*os.File, string, error) {
	path, err := creativeVideoCachePath(taskID)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, "", err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, "", err
	}
	return file, path, nil
}

func commitCreativeVideoCache(file *os.File, path string) error {
	if file == nil || strings.TrimSpace(path) == "" {
		return errors.New("creative video cache file is unavailable")
	}
	if err := file.Chmod(0o640); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func discardCreativeVideoCache(file *os.File) {
	if file == nil {
		return
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
}

func serveCreativeVideoCache(c *gin.Context, task *service.CreativeVideoTask, file *os.File, contentType string) {
	if c == nil || task == nil || file == nil {
		return
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "video/mp4"
	}
	c.Header("Content-Type", contentType)
	http.ServeContent(c.Writer, c.Request, task.TaskID+".mp4", fileInfoModTime(file), file)
}

func fileInfoModTime(file *os.File) time.Time {
	if file == nil {
		return time.Time{}
	}
	info, err := file.Stat()
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
