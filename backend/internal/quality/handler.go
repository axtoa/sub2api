package quality

import (
	"database/sql"
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
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Handler struct{ Runner *Runner }

func (h *Handler) Stop() { h.Runner.Stop() }

func authorized(c *gin.Context) bool {
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || role != "admin" {
		response.Forbidden(c, "需要管理员权限")
		return false
	}
	return true
}

func respondError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "监控记录不存在")
	} else if apperrors.Code(err) < 500 {
		response.ErrorFrom(c, err)
	} else {
		response.InternalError(c, "质量监控暂不可用，请稍后重试")
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

func validID(c *gin.Context) bool {
	if _, err := uuid.Parse(c.Param("id")); err != nil {
		response.BadRequest(c, "监控编号无效")
		return false
	}
	return true
}

func (h *Handler) Snapshot(c *gin.Context) {
	if !authorized(c) {
		return
	}
	ctx := c.Request.Context()
	cfg, err := h.Runner.Store.Settings(ctx)
	if respondError(c, err) {
		return
	}
	budget, err := h.Runner.Store.Budget(ctx, cfg, time.Now(), h.Runner.Location)
	if respondError(c, err) {
		return
	}
	targets, err := h.Runner.Store.Targets(ctx)
	if respondError(c, err) {
		return
	}
	if respondError(c, h.Runner.Store.Enrich(ctx, targets, time.Now().Add(-24*time.Hour))) {
		return
	}
	accounts, err := h.Runner.Store.Inventory(ctx)
	if respondError(c, err) {
		return
	}
	estimated := 0
	for _, t := range targets {
		if !t.Enabled || !t.AccountEnabled || !cfg.AutoEnabled {
			continue
		}
		p := t.Effective(cfg)
		if p.AvailabilityEnabled {
			estimated += (1440 + p.AvailabilityIntervalMinutes - 1) / p.AvailabilityIntervalMinutes
		}
		if p.QualityEnabled {
			estimated += (1440 + p.QualityIntervalMinutes - 1) / p.QualityIntervalMinutes * p.Questions
		}
		if p.CacheEnabled {
			estimated += (1440 + p.CacheIntervalMinutes - 1) / p.CacheIntervalMinutes * 3
		}
	}
	response.Success(c, gin.H{"settings": cfg, "defaults": DefaultSettings(), "budget": budget,
		"targets": targets, "accounts": accounts, "estimated_daily_requests": estimated, "server_time": time.Now()})
}

func (h *Handler) SyncAccounts(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var req struct {
		Initial bool `json:"initial"`
	}
	if !bind(c, &req) {
		return
	}
	created, err := h.Runner.Store.SyncAccounts(c.Request.Context(), req.Initial)
	if !respondError(c, err) {
		h.Runner.Wake()
		response.Success(c, gin.H{"created": created})
	}
}

func (h *Handler) SaveAccount(c *gin.Context) {
	if !authorized(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "账号编号无效")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
		Version int  `json:"version"`
	}
	if !bind(c, &req) {
		return
	}
	if !respondError(c, h.Runner.Store.SaveAccount(c.Request.Context(), id, req.Enabled, req.Version)) {
		h.Runner.Wake()
		response.Success(c, gin.H{"saved": true})
	}
}

func (h *Handler) SaveSettings(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var cfg Settings
	if !bind(c, &cfg) {
		return
	}
	saved, err := h.Runner.Store.SaveSettings(c.Request.Context(), cfg)
	if !respondError(c, err) {
		h.Runner.Wake()
		response.Success(c, saved)
	}
}

type accountChoice struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}

func (h *Handler) Accounts(c *gin.Context) {
	if !authorized(c) {
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		response.BadRequest(c, "搜索内容过长")
		return
	}
	rows, err := h.Runner.Store.DB.QueryContext(c.Request.Context(), `SELECT id,name,platform,status FROM accounts
 WHERE deleted_at IS NULL AND platform IN ('openai','anthropic') AND (type IN ('oauth','apikey') OR (platform='anthropic' AND type='setup-token'))
 AND ($1='' OR name ILIKE '%'||$1||'%') ORDER BY name,id LIMIT 200`, search)
	if respondError(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	items := []accountChoice{}
	for rows.Next() {
		var item accountChoice
		if respondError(c, rows.Scan(&item.ID, &item.Name, &item.Platform, &item.Status)) {
			return
		}
		items = append(items, item)
	}
	if !respondError(c, rows.Err()) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) CreateTargets(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var req struct {
		AccountIDs []int64  `json:"account_ids"`
		Models     []string `json:"models"`
	}
	if !bind(c, &req) {
		return
	}
	if len(req.AccountIDs) == 0 || len(req.Models) == 0 || len(req.AccountIDs)*len(req.Models) > 100 {
		response.BadRequest(c, "请选择账号与模型，每次最多添加 100 个组合")
		return
	}
	for _, model := range req.Models {
		if respondError(c, validateModel(model)) {
			return
		}
	}
	accounts := make(map[int64]*service.Account, len(req.AccountIDs))
	platform := ""
	for _, id := range req.AccountIDs {
		a, err := h.Runner.Accounts.GetByID(c.Request.Context(), id)
		if err != nil || a == nil {
			continue
		}
		if platform != "" && platform != a.Platform {
			response.BadRequest(c, "请分别添加 OpenAI 和 Anthropic 平台的账号模型，避免误发检测请求")
			return
		}
		platform, accounts[id] = a.Platform, a
	}
	cfg, err := h.Runner.Store.Settings(c.Request.Context())
	if respondError(c, err) {
		return
	}
	created := 0
	failures := []gin.H{}
	seen := map[string]bool{}
	for _, accountID := range req.AccountIDs {
		a := accounts[accountID]
		for _, model := range req.Models {
			key := strconv.FormatInt(accountID, 10) + ":" + model
			if seen[key] {
				continue
			}
			seen[key] = true
			if !service.QualityProbeSupported(a, model) {
				failures = append(failures, gin.H{"account_id": accountID, "model": model, "reason": "unsupported"})
				continue
			}
			_, err := h.Runner.Store.SaveTarget(c.Request.Context(), Target{AccountID: accountID, Model: model, Enabled: true}, cfg)
			if err == nil {
				created++
			} else if isConflict(err) {
				failures = append(failures, gin.H{"account_id": accountID, "model": model, "reason": "duplicate"})
			} else {
				respondError(c, err)
				return
			}
		}
	}
	h.Runner.Wake()
	response.Success(c, gin.H{"created": created, "failures": failures})
}

func (h *Handler) SaveTarget(c *gin.Context) {
	if !authorized(c) || !validID(c) {
		return
	}
	var req struct {
		Enabled   bool     `json:"enabled"`
		Overrides *Profile `json:"overrides"`
		Version   int      `json:"version"`
	}
	if !bind(c, &req) {
		return
	}
	t, err := h.Runner.Store.Target(c.Request.Context(), c.Param("id"))
	if respondError(c, err) {
		return
	}
	t.Enabled, t.Overrides, t.Version = req.Enabled, req.Overrides, req.Version
	cfg, err := h.Runner.Store.Settings(c.Request.Context())
	if respondError(c, err) {
		return
	}
	t, err = h.Runner.Store.SaveTarget(c.Request.Context(), t, cfg)
	if !respondError(c, err) {
		h.Runner.Wake()
		response.Success(c, t)
	}
}

func (h *Handler) Run(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var req struct {
		TargetIDs []string `json:"target_ids"`
		Kind      string   `json:"kind"`
	}
	if !bind(c, &req) {
		return
	}
	if !validKind(req.Kind) || len(req.TargetIDs) == 0 || len(req.TargetIDs) > 100 {
		response.BadRequest(c, "请选择检测类型与最多 100 个账号模型")
		return
	}
	for _, id := range req.TargetIDs {
		if _, err := uuid.Parse(id); err != nil {
			response.BadRequest(c, "监控编号无效")
			return
		}
	}

	queued, skipped := 0, 0
	jobs := []Run{}
	seen := map[string]bool{}
	for _, id := range req.TargetIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		t, err := h.Runner.Store.Target(c.Request.Context(), id)
		if err != nil {
			skipped++
			continue
		}
		a, err := h.Runner.Accounts.GetByID(c.Request.Context(), t.AccountID)
		if err != nil || !service.QualityProbeSupported(a, t.Model) {
			skipped++
			continue
		}
		job, err := h.Runner.Store.Queue(c.Request.Context(), id, req.Kind, true)
		if err != nil {
			skipped++
			continue
		}
		queued++
		jobs = append(jobs, job)
	}
	h.Runner.Wake()
	response.Accepted(c, gin.H{"queued": queued, "skipped": skipped, "jobs": jobs})
}

// Batch status returns only summaries, never the potentially large question bodies.
func (h *Handler) RunStatuses(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if !bind(c, &req) {
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > 100 {
		response.BadRequest(c, "每次最多查询 100 条检测进度")
		return
	}
	for _, id := range req.IDs {
		if _, err := uuid.Parse(id); err != nil {
			response.BadRequest(c, "检测编号无效")
			return
		}
	}
	columns := strings.Replace(runColumns, "result,", "(result - 'items' - 'cache_prefix') AS result,", 1)
	rows, err := h.Runner.Store.DB.QueryContext(c.Request.Context(), `SELECT `+columns+` FROM quality_monitor_runs WHERE id=ANY($1::uuid[])`, pq.Array(req.IDs))
	if respondError(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	items := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if respondError(c, err) {
			return
		}
		r.Profile = nil
		items = append(items, r)
	}
	if !respondError(c, rows.Err()) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) History(c *gin.Context) {
	if !authorized(c) || !validID(c) {
		return
	}
	items, err := h.Runner.Store.History(c.Request.Context(), c.Param("id"), 60)
	if respondError(c, err) {
		return
	}
	for i := range items {
		if items[i].Result != nil {
			items[i].Result.Items = nil
			items[i].Result.CachePrefix = ""
		}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) Detail(c *gin.Context) {
	if !authorized(c) || !validID(c) {
		return
	}
	r, err := scanRun(h.Runner.Store.DB.QueryRowContext(c.Request.Context(), `SELECT `+runColumns+` FROM quality_monitor_runs WHERE id=$1`, c.Param("id")))
	if !respondError(c, err) {
		response.Success(c, r)
	}
}
