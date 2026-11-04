package quality

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ExternalTool struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Kind         string   `json:"kind"`
	Families     []string `json:"families"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	Usage        string   `json:"usage,omitempty"`
	Origin       string   `json:"origin,omitempty"`
	Review       string   `json:"review,omitempty"`
	ReviewedAt   string   `json:"reviewed_at,omitempty"`
	ReferenceURL string   `json:"reference_url,omitempty"`
}

type ExternalTools struct {
	Items   []ExternalTool `json:"items"`
	Version int            `json:"version"`
}

var externalToolID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func validExternalURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil && len(value) <= 2048 && !strings.ContainsAny(value, "\r\n\t")
}

func externalKind(kind string) bool {
	switch kind {
	case "channel", "benchmark", "ranking", "status", "pricing", "upstream", "payment", "email", "infrastructure", "operations", "marketplace", "creative":
		return true
	}
	return false
}

func (list ExternalTools) validate() error {
	if list.Version < 1 || len(list.Items) > 100 {
		return invalid("最多保存 100 个资源，请刷新后重试")
	}
	seen := map[string]bool{}
	for _, tool := range list.Items {
		if !externalToolID.MatchString(tool.ID) || seen[tool.ID] {
			return invalid("网站编号无效或重复")
		}
		seen[tool.ID] = true
		if strings.TrimSpace(tool.Name) == "" || utf8.RuneCountInString(tool.Name) > 100 || utf8.RuneCountInString(tool.Description) > 2000 {
			return invalid("请填写网站名称，名称最多 100 字，介绍最多 2000 字")
		}
		if !validExternalURL(tool.URL) || (tool.ReferenceURL != "" && !validExternalURL(tool.ReferenceURL)) {
			return invalid("请填写不包含账号密码的完整 HTTP 或 HTTPS 网址")
		}
		if !externalKind(tool.Kind) {
			return invalid("请选择网站类型")
		}
		if len(tool.Families) > 2 {
			return invalid("请选择适用模型")
		}
		families := map[string]bool{}
		for _, family := range tool.Families {
			if (family != "gpt" && family != "claude") || families[family] {
				return invalid("适用模型无效或重复")
			}
			families[family] = true
		}
		if utf8.RuneCountInString(tool.Notes) > 2000 || len(tool.Tags) > 8 {
			return invalid("备注最多 2000 字，标签最多 8 个")
		}
		tags := map[string]bool{}
		for _, tag := range tool.Tags {
			if strings.TrimSpace(tag) == "" || utf8.RuneCountInString(tag) > 30 || tags[tag] {
				return invalid("标签不可为空或重复，每个最多 30 字")
			}
			tags[tag] = true
		}
		if tool.Usage != "" && tool.Usage != "saved" && tool.Usage != "trial" && tool.Usage != "using" {
			return invalid("请选择使用情况")
		}
		if tool.Origin != "" && tool.Origin != "user" && tool.Origin != "curated" && tool.Origin != "custom" {
			return invalid("请选择资源来源")
		}
		if tool.Review != "" && tool.Review != "public" && tool.Review != "unconfirmed" {
			return invalid("请选择资料核对状态")
		}
		if tool.ReviewedAt != "" {
			if _, err := time.Parse("2006-01-02", tool.ReviewedAt); err != nil {
				return invalid("核对日期格式应为 YYYY-MM-DD")
			}
		}
	}
	raw, err := json.Marshal(list.Items)
	if err != nil {
		return err
	}
	if len(raw) > 256<<10 {
		return invalid("网站列表内容最多 256 KB")
	}
	return nil
}

func (s *Store) ExternalTools(ctx context.Context) (ExternalTools, error) {
	list := ExternalTools{Items: []ExternalTool{}}
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT items,version FROM quality_external_tools WHERE id=TRUE`).Scan(&raw, &list.Version)
	if err == nil {
		err = json.Unmarshal(raw, &list.Items)
	}
	return list, err
}

func (s *Store) SaveExternalTools(ctx context.Context, list ExternalTools) (ExternalTools, error) {
	if err := list.validate(); err != nil {
		return list, err
	}
	if list.Items == nil {
		list.Items = []ExternalTool{}
	}
	raw, err := json.Marshal(list.Items)
	if err != nil {
		return list, err
	}
	err = s.DB.QueryRowContext(ctx, `UPDATE quality_external_tools SET items=$1,version=version+1,updated_at=NOW() WHERE id=TRUE AND version=$2 RETURNING version`, string(raw), list.Version).Scan(&list.Version)
	if errors.Is(err, sql.ErrNoRows) {
		err = conflict("网站列表已被其他窗口修改，请重新载入后再编辑")
	}
	return list, err
}

func (h *Handler) ExternalTools(c *gin.Context) {
	if !authorized(c) {
		return
	}
	list, err := h.Runner.Store.ExternalTools(c.Request.Context())
	if !respondError(c, err) {
		response.Success(c, list)
	}
}

func (h *Handler) SaveExternalTools(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var list ExternalTools
	if !bindLimit(c, &list, 512<<10) {
		return
	}
	saved, err := h.Runner.Store.SaveExternalTools(c.Request.Context(), list)
	if !respondError(c, err) {
		response.Success(c, saved)
	}
}
