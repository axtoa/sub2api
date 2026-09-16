package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	CreativeVideoProviderOpenAI  = "openai"
	CreativeVideoProviderMiniMax = "minimax"
)

var (
	ErrCreativeVideoProviderUnsupportedAccount = infraerrors.New(http.StatusBadRequest, "CREATIVE_VIDEO_PROVIDER_UNSUPPORTED_ACCOUNT", "creative video provider does not support this account")
	ErrCreativeVideoProviderMissingAPIKey      = infraerrors.New(http.StatusBadRequest, "CREATIVE_VIDEO_PROVIDER_MISSING_API_KEY", "creative video provider account is missing api key")
	ErrCreativeVideoProviderInvalidInput       = infraerrors.New(http.StatusBadRequest, "CREATIVE_VIDEO_PROVIDER_INVALID_INPUT", "invalid creative video provider input")
	ErrCreativeVideoProviderOutputUnavailable  = infraerrors.New(http.StatusBadRequest, "CREATIVE_VIDEO_OUTPUT_UNAVAILABLE", "creative video output is not ready")
)

type CreativeVideoProviderRequest struct {
	Model       string
	Prompt      string
	AspectRatio string
	Resolution  string
	Duration    int
	ImageURL    string
}

type CreativeVideoProviderStatus struct {
	ID              string
	Status          string
	Model           string
	Resolution      string
	DurationSeconds int
	DownloadURL     string
	FileID          string
	Raw             map[string]any
}

type CreativeVideoHTTPProvider struct {
	platform string
	client   *http.Client
}

func NewCreativeVideoHTTPProvider(platform string, client *http.Client) *CreativeVideoHTTPProvider {
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	return &CreativeVideoHTTPProvider{
		platform: NormalizeGroupPlatform(platform),
		client:   client,
	}
}

func (p *CreativeVideoHTTPProvider) Name() string {
	return p.platform
}

func (p *CreativeVideoHTTPProvider) SupportsAccount(account *Account) bool {
	if account == nil || batchImageProviderAPIKey(account) == "" {
		return false
	}
	switch p.platform {
	case PlatformOpenAI:
		return account.Platform == PlatformOpenAI && account.Type == AccountTypeAPIKey
	case PlatformMiniMax:
		return account.Platform == PlatformMiniMax && (account.Type == AccountTypeAPIKey || account.Type == AccountTypeUpstream)
	default:
		return false
	}
}

func (p *CreativeVideoHTTPProvider) Submit(ctx context.Context, account *Account, req CreativeVideoProviderRequest) (*CreativeVideoProviderStatus, error) {
	if !p.SupportsAccount(account) {
		return nil, ErrCreativeVideoProviderUnsupportedAccount
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Model == "" || req.Prompt == "" {
		return nil, ErrCreativeVideoProviderInvalidInput.WithCause(fmt.Errorf("model and prompt are required"))
	}
	client := newCreativeVideoHTTPClient(account, p.platform, p.client)
	switch p.platform {
	case PlatformOpenAI:
		return p.submitOpenAI(ctx, client, req)
	case PlatformMiniMax:
		return p.submitMiniMax(ctx, client, req)
	default:
		return nil, ErrCreativeVideoProviderUnsupportedAccount
	}
}

func (p *CreativeVideoHTTPProvider) Get(ctx context.Context, account *Account, requestID string) (*CreativeVideoProviderStatus, error) {
	if !p.SupportsAccount(account) {
		return nil, ErrCreativeVideoProviderUnsupportedAccount
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, ErrCreativeVideoProviderInvalidInput.WithCause(fmt.Errorf("request id is required"))
	}
	client := newCreativeVideoHTTPClient(account, p.platform, p.client)
	switch p.platform {
	case PlatformOpenAI:
		var resp map[string]any
		if err := client.doJSON(ctx, http.MethodGet, "/v1/videos/"+requestID, nil, &resp); err != nil {
			return nil, err
		}
		return normalizeOpenAIVideoStatus(resp), nil
	case PlatformMiniMax:
		var resp map[string]any
		path := "/v1/query/video_generation?task_id=" + requestID
		if err := client.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		if err := minimaxBaseResponseError(resp); err != nil {
			return nil, err
		}
		return normalizeMiniMaxVideoStatus(resp, requestID), nil
	default:
		return nil, ErrCreativeVideoProviderUnsupportedAccount
	}
}

func (p *CreativeVideoHTTPProvider) OpenContent(ctx context.Context, account *Account, status *CreativeVideoProviderStatus) (io.ReadCloser, string, error) {
	if !p.SupportsAccount(account) {
		return nil, "", ErrCreativeVideoProviderUnsupportedAccount
	}
	if status == nil || strings.TrimSpace(status.ID) == "" {
		return nil, "", ErrCreativeVideoProviderInvalidInput.WithCause(fmt.Errorf("request id is required"))
	}
	client := newCreativeVideoHTTPClient(account, p.platform, p.client)
	switch p.platform {
	case PlatformOpenAI:
		return client.open(ctx, "/v1/videos/"+strings.TrimSpace(status.ID)+"/content")
	case PlatformMiniMax:
		downloadURL := strings.TrimSpace(status.DownloadURL)
		if downloadURL == "" && strings.TrimSpace(status.FileID) != "" {
			url, err := p.miniMaxDownloadURL(ctx, client, status.FileID)
			if err != nil {
				return nil, "", err
			}
			downloadURL = url
		}
		if downloadURL == "" {
			return nil, "", ErrCreativeVideoProviderOutputUnavailable
		}
		return client.openAbsolute(ctx, downloadURL)
	default:
		return nil, "", ErrCreativeVideoProviderUnsupportedAccount
	}
}

func (p *CreativeVideoHTTPProvider) submitOpenAI(ctx context.Context, client *creativeVideoHTTPClient, req CreativeVideoProviderRequest) (*CreativeVideoProviderStatus, error) {
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	if size := creativeVideoOpenAISize(req.AspectRatio, req.Resolution); size != "" {
		payload["size"] = size
	}
	if req.Duration > 0 {
		payload["seconds"] = req.Duration
	}
	if image := strings.TrimSpace(req.ImageURL); image != "" {
		payload["image"] = image
	}
	var resp map[string]any
	if err := client.doJSON(ctx, http.MethodPost, "/v1/videos", payload, &resp); err != nil {
		return nil, err
	}
	return normalizeOpenAIVideoStatus(resp), nil
}

func (p *CreativeVideoHTTPProvider) submitMiniMax(ctx context.Context, client *creativeVideoHTTPClient, req CreativeVideoProviderRequest) (*CreativeVideoProviderStatus, error) {
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	if req.Duration > 0 {
		payload["duration"] = req.Duration
	}
	if resolution := strings.TrimSpace(req.Resolution); resolution != "" {
		payload["resolution"] = resolution
	}
	if ratio := strings.TrimSpace(req.AspectRatio); ratio != "" {
		payload["aspect_ratio"] = ratio
	}
	if image := strings.TrimSpace(req.ImageURL); image != "" {
		payload["first_frame_image"] = image
		payload["image"] = image
	}
	var resp map[string]any
	if err := client.doJSON(ctx, http.MethodPost, "/v1/video_generation", payload, &resp); err != nil {
		return nil, err
	}
	if err := minimaxBaseResponseError(resp); err != nil {
		return nil, err
	}
	return normalizeMiniMaxVideoStatus(resp, ""), nil
}

func (p *CreativeVideoHTTPProvider) miniMaxDownloadURL(ctx context.Context, client *creativeVideoHTTPClient, fileID string) (string, error) {
	var resp map[string]any
	path := "/v1/files/retrieve?file_id=" + strings.TrimSpace(fileID)
	if err := client.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return "", err
	}
	if err := minimaxBaseResponseError(resp); err != nil {
		return "", err
	}
	return firstNonEmptyString(
		gjson.GetBytes(mustJSON(resp), "file.download_url").String(),
		gjson.GetBytes(mustJSON(resp), "file.url").String(),
		gjson.GetBytes(mustJSON(resp), "download_url").String(),
		gjson.GetBytes(mustJSON(resp), "url").String(),
	), nil
}

type creativeVideoHTTPClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newCreativeVideoHTTPClient(account *Account, platform string, client *http.Client) *creativeVideoHTTPClient {
	baseURL := ""
	if account != nil {
		baseURL = account.GetOpenAIBaseURL()
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		switch NormalizeGroupPlatform(platform) {
		case PlatformMiniMax:
			baseURL = "https://api.minimax.io"
		default:
			baseURL = "https://api.openai.com"
		}
	}
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	return &creativeVideoHTTPClient{baseURL: baseURL, apiKey: batchImageProviderAPIKey(account), client: client}
}

func (c *creativeVideoHTTPClient) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

func (c *creativeVideoHTTPClient) open(ctx context.Context, path string) (io.ReadCloser, string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	return c.doOpen(req)
}

func (c *creativeVideoHTTPClient) openAbsolute(ctx context.Context, rawURL string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	return c.doOpen(req)
}

func (c *creativeVideoHTTPClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, ErrCreativeVideoProviderMissingAPIKey
	}
	req, err := http.NewRequestWithContext(ctx, method, batchImageProviderJoinURL(c.baseURL, path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *creativeVideoHTTPClient) do(req *http.Request, out any) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return fmt.Errorf("upstream status %d: %s", resp.StatusCode, truncateBatchImageMessage(string(data), 500))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *creativeVideoHTTPClient) doOpen(req *http.Request) (io.ReadCloser, string, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer func() { _ = resp.Body.Close() }()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, "", fmt.Errorf("upstream status %d: %s", resp.StatusCode, truncateBatchImageMessage(string(data), 500))
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "video/mp4"
	}
	return resp.Body, contentType, nil
}

func normalizeOpenAIVideoStatus(resp map[string]any) *CreativeVideoProviderStatus {
	data := mustJSON(resp)
	status := strings.ToLower(firstNonEmptyString(
		gjson.GetBytes(data, "status").String(),
		gjson.GetBytes(data, "state").String(),
	))
	out := &CreativeVideoProviderStatus{
		ID:              firstNonEmptyString(gjson.GetBytes(data, "id").String(), gjson.GetBytes(data, "request_id").String()),
		Status:          normalizeCreativeVideoProviderStatus(status, false),
		Model:           gjson.GetBytes(data, "model").String(),
		Resolution:      firstNonEmptyString(gjson.GetBytes(data, "size").String(), gjson.GetBytes(data, "resolution").String()),
		DurationSeconds: int(gjson.GetBytes(data, "seconds").Int()),
		Raw:             resp,
	}
	return out
}

func normalizeMiniMaxVideoStatus(resp map[string]any, fallbackID string) *CreativeVideoProviderStatus {
	data := mustJSON(resp)
	status := strings.ToLower(firstNonEmptyString(
		gjson.GetBytes(data, "status").String(),
		gjson.GetBytes(data, "data.status").String(),
		gjson.GetBytes(data, "task_status").String(),
		gjson.GetBytes(data, "data.task_status").String(),
	))
	fileID := firstNonEmptyString(
		gjson.GetBytes(data, "file_id").String(),
		gjson.GetBytes(data, "data.file_id").String(),
		gjson.GetBytes(data, "video.file_id").String(),
		gjson.GetBytes(data, "data.video.file_id").String(),
	)
	downloadURL := firstNonEmptyString(
		gjson.GetBytes(data, "download_url").String(),
		gjson.GetBytes(data, "data.download_url").String(),
		gjson.GetBytes(data, "video.url").String(),
		gjson.GetBytes(data, "data.video.url").String(),
	)
	done := fileID != "" || downloadURL != ""
	return &CreativeVideoProviderStatus{
		ID: firstNonEmptyString(
			gjson.GetBytes(data, "task_id").String(),
			gjson.GetBytes(data, "data.task_id").String(),
			gjson.GetBytes(data, "id").String(),
			fallbackID,
		),
		Status:          normalizeCreativeVideoProviderStatus(status, done),
		Model:           firstNonEmptyString(gjson.GetBytes(data, "model").String(), gjson.GetBytes(data, "data.model").String()),
		Resolution:      firstNonEmptyString(gjson.GetBytes(data, "resolution").String(), gjson.GetBytes(data, "data.resolution").String()),
		DurationSeconds: int(firstPositive(int(gjson.GetBytes(data, "duration").Int()), int(gjson.GetBytes(data, "data.duration").Int()))),
		DownloadURL:     downloadURL,
		FileID:          fileID,
		Raw:             resp,
	}
}

func normalizeCreativeVideoProviderStatus(status string, done bool) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "complete", "succeeded", "success", "done":
		return CreativeVideoStatusCompleted
	case "failed", "error", "fail":
		return CreativeVideoStatusFailed
	case "expired":
		return CreativeVideoStatusExpired
	case "queued", "pending", "running", "processing", "in_progress":
		return CreativeVideoStatusRunning
	default:
		if done {
			return CreativeVideoStatusCompleted
		}
		return CreativeVideoStatusRunning
	}
}

func creativeVideoOpenAISize(aspectRatio, resolution string) string {
	tier := NormalizeVideoBillingResolutionOrDefault(resolution)
	switch strings.TrimSpace(aspectRatio) {
	case "9:16":
		if tier == VideoBillingResolution1080P {
			return "1080x1920"
		}
		return "720x1280"
	default:
		if tier == VideoBillingResolution1080P {
			return "1920x1080"
		}
		return "1280x720"
	}
}

func CreativeVideoProviderResponseJSON(status *CreativeVideoProviderStatus) map[string]any {
	if status == nil {
		return map[string]any{}
	}
	upstreamStatus := status.Status
	if status.Status == CreativeVideoStatusCompleted {
		upstreamStatus = "done"
	}
	resp := map[string]any{
		"id":     status.ID,
		"object": "video",
		"status": upstreamStatus,
		"model":  status.Model,
	}
	if status.DurationSeconds > 0 {
		resp["video"] = map[string]any{"duration": status.DurationSeconds}
	}
	if strings.TrimSpace(status.Resolution) != "" {
		resp["resolution"] = status.Resolution
	}
	return resp
}

func mustJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}

func CreativeVideoForwardResultFromStatus(status *CreativeVideoProviderStatus, requestID string) *OpenAIForwardResult {
	if status == nil || status.Status != CreativeVideoStatusCompleted {
		return nil
	}
	duration := status.DurationSeconds
	if duration <= 0 {
		duration = NormalizeVideoBillingDurationSecondsOrDefault(0)
	}
	return &OpenAIForwardResult{
		RequestID:            StableGrokVideoBillingRequestID(firstNonEmptyString(requestID, status.ID)),
		ResponseID:           firstNonEmptyString(status.ID, requestID),
		Model:                strings.TrimSpace(status.Model),
		BillingModel:         strings.TrimSpace(status.Model),
		VideoCount:           1,
		VideoStatus:          "done",
		VideoResolution:      NormalizeVideoBillingResolutionOrDefault(status.Resolution),
		VideoDurationSeconds: duration,
		Duration:             time.Duration(duration) * time.Second,
	}
}
