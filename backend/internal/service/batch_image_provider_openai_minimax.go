package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultOpenAIImageRequeueAfter  = time.Second
	defaultMiniMaxImageRequeueAfter = time.Second
)

type OpenAIBatchImageProviderOptions struct {
	ResultDir string
}

type MiniMaxBatchImageProviderOptions struct {
	ResultDir string
}

type OpenAIBatchImageProvider struct {
	client    *http.Client
	resultDir string
}

type MiniMaxBatchImageProvider struct {
	client    *http.Client
	resultDir string
}

type batchImageHTTPClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type providerImageResult struct {
	MimeType string
	Data     []byte
}

func NewOpenAIBatchImageProvider(opts OpenAIBatchImageProviderOptions, client *http.Client) *OpenAIBatchImageProvider {
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	return &OpenAIBatchImageProvider{client: client, resultDir: strings.TrimSpace(opts.ResultDir)}
}

func NewMiniMaxBatchImageProvider(opts MiniMaxBatchImageProviderOptions, client *http.Client) *MiniMaxBatchImageProvider {
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	return &MiniMaxBatchImageProvider{client: client, resultDir: strings.TrimSpace(opts.ResultDir)}
}

func (p *OpenAIBatchImageProvider) Name() string {
	return BatchImageProviderOpenAI
}

func (p *OpenAIBatchImageProvider) SupportsAccount(account *Account) bool {
	return account != nil &&
		account.Platform == PlatformOpenAI &&
		account.Type == AccountTypeAPIKey &&
		batchImageProviderAPIKey(account) != ""
}

func (p *OpenAIBatchImageProvider) Submit(ctx context.Context, job *BatchImageJob, account *Account, input BatchImageInput) (*BatchProviderJob, error) {
	if !p.SupportsAccount(account) {
		return nil, ErrBatchImageProviderUnsupportedAccount
	}
	if err := normalizeBatchImageProviderInput(job, &input); err != nil {
		return nil, err
	}
	client := newBatchImageHTTPClient(account.GetOpenAIBaseURL(), batchImageProviderAPIKey(account), p.client)
	resultRef, err := p.writeResult(ctx, input.BatchID, func(w io.Writer) error {
		for _, item := range input.Items {
			images, err := p.generateItem(ctx, client, input, item)
			if err != nil {
				if encErr := writeBatchImageProviderFailureLine(w, item.CustomID, "OPENAI_IMAGE_FAILED", sanitizeBatchImagePublicMessage(err.Error())); encErr != nil {
					return encErr
				}
				continue
			}
			if err := writeBatchImageProviderSuccessLine(w, item.CustomID, images); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, openAIImageProviderError("OPENAI_IMAGE_SUBMIT_FAILED", "OpenAI image generation failed", err)
	}
	return &BatchProviderJob{
		ProviderJobName:   input.BatchID,
		ProviderOutputRef: resultRef,
		RawState:          string(BatchProviderStateSucceeded),
	}, nil
}

func (p *OpenAIBatchImageProvider) Get(ctx context.Context, job *BatchImageJob, account *Account) (*BatchProviderStatus, error) {
	if !p.SupportsAccount(account) {
		return nil, ErrBatchImageProviderUnsupportedAccount
	}
	if _, err := p.resultPath(batchImageProviderOutputRef(job)); err != nil {
		return nil, err
	}
	return &BatchProviderStatus{
		RawState:              string(BatchProviderStateSucceeded),
		InternalState:         BatchProviderStateSucceeded,
		Done:                  true,
		ProviderOutputRef:     batchImageProviderOutputRef(job),
		SuggestedRequeueAfter: defaultOpenAIImageRequeueAfter,
	}, nil
}

func (p *OpenAIBatchImageProvider) Cancel(context.Context, *BatchImageJob, *Account) error {
	return nil
}

func (p *OpenAIBatchImageProvider) OpenResult(_ context.Context, job *BatchImageJob, account *Account) (io.ReadCloser, string, error) {
	if !p.SupportsAccount(account) {
		return nil, "", ErrBatchImageProviderUnsupportedAccount
	}
	path, err := p.resultPath(batchImageProviderOutputRef(job))
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	return f, "application/jsonl", nil
}

func (p *OpenAIBatchImageProvider) Cleanup(_ context.Context, job *BatchImageJob, account *Account, target CleanupTarget) error {
	if !p.SupportsAccount(account) {
		return ErrBatchImageProviderUnsupportedAccount
	}
	if target != CleanupTargetOutput && target != CleanupTargetAll {
		return nil
	}
	return removeBatchImageLocalResult(p.resultDir, batchImageProviderOutputRef(job))
}

func (p *OpenAIBatchImageProvider) generateItem(ctx context.Context, client *batchImageHTTPClient, input BatchImageInput, item BatchImageInputItem) ([]providerImageResult, error) {
	if len(item.ReferenceImages) > 0 {
		return p.editItem(ctx, client, input, item)
	}
	payload := map[string]any{
		"model":  input.Model,
		"prompt": item.Prompt,
		"n":      1,
		"size":   batchImageOpenAIOutputSize(input.AspectRatio),
	}
	if format := batchImageOpenAIOutputFormat(input.ResponseMimeType); format != "" {
		payload["output_format"] = format
	}
	var resp map[string]any
	if err := client.doJSON(ctx, http.MethodPost, "/v1/images/generations", payload, &resp); err != nil {
		return nil, err
	}
	return extractProviderImageResults(ctx, p.client, resp)
}

func (p *OpenAIBatchImageProvider) editItem(ctx context.Context, client *batchImageHTTPClient, input BatchImageInput, item BatchImageInputItem) ([]providerImageResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"model":  input.Model,
		"prompt": item.Prompt,
		"n":      "1",
		"size":   batchImageOpenAIOutputSize(input.AspectRatio),
	}
	if format := batchImageOpenAIOutputFormat(input.ResponseMimeType); format != "" {
		fields["output_format"] = format
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	for index, ref := range item.ReferenceImages {
		if len(ref.Data) == 0 {
			return nil, batchImageProviderInputError("OpenAI image edit requires inline reference image data")
		}
		mimeType := normalizeBatchImageReferenceMimeType(ref.MimeType)
		if mimeType == "" {
			return nil, batchImageProviderInputError("reference image mime_type is required")
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="reference-%d%s"`, index+1, batchImageFileExtensionWithDot(mimeType)))
		header.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(ref.Data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	var resp map[string]any
	if err := client.doMultipart(ctx, "/v1/images/edits", writer.FormDataContentType(), body.Bytes(), &resp); err != nil {
		return nil, err
	}
	return extractProviderImageResults(ctx, p.client, resp)
}

func (p *OpenAIBatchImageProvider) writeResult(ctx context.Context, batchID string, fn func(io.Writer) error) (string, error) {
	return writeBatchImageLocalResult(ctx, p.resultDir, BatchImageProviderOpenAI, batchID, fn)
}

func (p *OpenAIBatchImageProvider) resultPath(ref string) (string, error) {
	return batchImageLocalResultPath(p.resultDir, ref)
}

func (p *MiniMaxBatchImageProvider) Name() string {
	return BatchImageProviderMiniMax
}

func (p *MiniMaxBatchImageProvider) SupportsAccount(account *Account) bool {
	return account != nil &&
		account.Platform == PlatformMiniMax &&
		(account.Type == AccountTypeAPIKey || account.Type == AccountTypeUpstream) &&
		batchImageProviderAPIKey(account) != ""
}

func (p *MiniMaxBatchImageProvider) Submit(ctx context.Context, job *BatchImageJob, account *Account, input BatchImageInput) (*BatchProviderJob, error) {
	if !p.SupportsAccount(account) {
		return nil, ErrBatchImageProviderUnsupportedAccount
	}
	if err := normalizeBatchImageProviderInput(job, &input); err != nil {
		return nil, err
	}
	client := newBatchImageHTTPClient(account.GetOpenAIBaseURL(), batchImageProviderAPIKey(account), p.client)
	resultRef, err := p.writeResult(ctx, input.BatchID, func(w io.Writer) error {
		for _, item := range input.Items {
			images, err := p.generateItem(ctx, client, input, item)
			if err != nil {
				if encErr := writeBatchImageProviderFailureLine(w, item.CustomID, "MINIMAX_IMAGE_FAILED", sanitizeBatchImagePublicMessage(err.Error())); encErr != nil {
					return encErr
				}
				continue
			}
			if err := writeBatchImageProviderSuccessLine(w, item.CustomID, images); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, minimaxImageProviderError("MINIMAX_IMAGE_SUBMIT_FAILED", "MiniMax image generation failed", err)
	}
	return &BatchProviderJob{
		ProviderJobName:   input.BatchID,
		ProviderOutputRef: resultRef,
		RawState:          string(BatchProviderStateSucceeded),
	}, nil
}

func (p *MiniMaxBatchImageProvider) Get(ctx context.Context, job *BatchImageJob, account *Account) (*BatchProviderStatus, error) {
	if !p.SupportsAccount(account) {
		return nil, ErrBatchImageProviderUnsupportedAccount
	}
	if _, err := p.resultPath(batchImageProviderOutputRef(job)); err != nil {
		return nil, err
	}
	return &BatchProviderStatus{
		RawState:              string(BatchProviderStateSucceeded),
		InternalState:         BatchProviderStateSucceeded,
		Done:                  true,
		ProviderOutputRef:     batchImageProviderOutputRef(job),
		SuggestedRequeueAfter: defaultMiniMaxImageRequeueAfter,
	}, nil
}

func (p *MiniMaxBatchImageProvider) Cancel(context.Context, *BatchImageJob, *Account) error {
	return nil
}

func (p *MiniMaxBatchImageProvider) OpenResult(_ context.Context, job *BatchImageJob, account *Account) (io.ReadCloser, string, error) {
	if !p.SupportsAccount(account) {
		return nil, "", ErrBatchImageProviderUnsupportedAccount
	}
	path, err := p.resultPath(batchImageProviderOutputRef(job))
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	return f, "application/jsonl", nil
}

func (p *MiniMaxBatchImageProvider) Cleanup(_ context.Context, job *BatchImageJob, account *Account, target CleanupTarget) error {
	if !p.SupportsAccount(account) {
		return ErrBatchImageProviderUnsupportedAccount
	}
	if target != CleanupTargetOutput && target != CleanupTargetAll {
		return nil
	}
	return removeBatchImageLocalResult(p.resultDir, batchImageProviderOutputRef(job))
}

func (p *MiniMaxBatchImageProvider) generateItem(ctx context.Context, client *batchImageHTTPClient, input BatchImageInput, item BatchImageInputItem) ([]providerImageResult, error) {
	payload := map[string]any{
		"model":  input.Model,
		"prompt": item.Prompt,
		"n":      1,
	}
	if size := batchImageMiniMaxSize(input.AspectRatio); size != "" {
		payload["size"] = size
	}
	if len(item.ReferenceImages) > 0 {
		refs := make([]string, 0, len(item.ReferenceImages))
		for _, ref := range item.ReferenceImages {
			mimeType := normalizeBatchImageReferenceMimeType(ref.MimeType)
			if mimeType == "" || len(ref.Data) == 0 {
				return nil, batchImageProviderInputError("MiniMax image edit requires inline reference image data")
			}
			refs = append(refs, fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(ref.Data)))
		}
		payload["image"] = refs[0]
		payload["images"] = refs
		payload["reference_images"] = refs
	}
	var resp map[string]any
	if err := client.doJSON(ctx, http.MethodPost, "/v1/image_generation", payload, &resp); err != nil {
		return nil, err
	}
	if err := minimaxBaseResponseError(resp); err != nil {
		return nil, err
	}
	return extractProviderImageResults(ctx, p.client, resp)
}

func (p *MiniMaxBatchImageProvider) writeResult(ctx context.Context, batchID string, fn func(io.Writer) error) (string, error) {
	return writeBatchImageLocalResult(ctx, p.resultDir, BatchImageProviderMiniMax, batchID, fn)
}

func (p *MiniMaxBatchImageProvider) resultPath(ref string) (string, error) {
	return batchImageLocalResultPath(p.resultDir, ref)
}

func normalizeBatchImageProviderInput(job *BatchImageJob, input *BatchImageInput) error {
	if input == nil {
		return batchImageProviderInputError("input is required")
	}
	if strings.TrimSpace(input.BatchID) == "" && job != nil {
		input.BatchID = job.BatchID
	}
	if strings.TrimSpace(input.Model) == "" && job != nil {
		input.Model = job.Model
	}
	if strings.TrimSpace(input.BatchID) == "" {
		return batchImageProviderInputError("batch_id is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		return batchImageProviderInputError("model is required")
	}
	if len(input.Items) == 0 {
		return batchImageProviderInputError("at least one item is required")
	}
	return nil
}

func newBatchImageHTTPClient(baseURL, apiKey string, client *http.Client) *batchImageHTTPClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	return &batchImageHTTPClient{baseURL: baseURL, apiKey: strings.TrimSpace(apiKey), client: client}
}

func (c *batchImageHTTPClient) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *batchImageHTTPClient) doMultipart(ctx context.Context, path, contentType string, body []byte, out any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	return c.do(req, out)
}

func (c *batchImageHTTPClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, ErrBatchImageProviderMissingAPIKey
	}
	u := batchImageProviderJoinURL(c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func batchImageProviderJoinURL(baseURL, path string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	p := "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(p, "/v1/") {
		p = strings.TrimPrefix(p, "/v1")
	}
	return base + p
}

func (c *batchImageHTTPClient) do(req *http.Request, out any) error {
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

func writeBatchImageLocalResult(ctx context.Context, root, provider, batchID string, fn func(io.Writer) error) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = batchImageProviderLocalResultDir(nil)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	ref := fmt.Sprintf("local-jsonl:%s/%s.jsonl", provider, batchID)
	path, err := batchImageLocalResultPath(root, ref)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", err
	}
	writeErr := fn(f)
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return "", writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return ref, nil
}

func batchImageLocalResultPath(root, ref string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = batchImageProviderLocalResultDir(nil)
	}
	const prefix = "local-jsonl:"
	if !strings.HasPrefix(ref, prefix) {
		return "", ErrBatchImageProviderMissingResultRef
	}
	rel := strings.TrimPrefix(ref, prefix)
	if strings.TrimSpace(rel) == "" || filepath.IsAbs(rel) {
		return "", ErrBatchImageProviderUnsafeCleanupPath
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == "." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) || cleanRel == ".." {
		return "", ErrBatchImageProviderUnsafeCleanupPath
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(filepath.Join(rootAbs, cleanRel))
	if err != nil {
		return "", err
	}
	if pathAbs != rootAbs && !strings.HasPrefix(pathAbs, rootAbs+string(os.PathSeparator)) {
		return "", ErrBatchImageProviderUnsafeCleanupPath
	}
	return pathAbs, nil
}

func removeBatchImageLocalResult(root, ref string) error {
	if strings.TrimSpace(ref) == "" {
		return nil
	}
	path, err := batchImageLocalResultPath(root, ref)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func writeBatchImageProviderSuccessLine(w io.Writer, customID string, images []providerImageResult) error {
	if strings.TrimSpace(customID) == "" {
		return batchImageProviderInputError("custom_id is required")
	}
	if len(images) == 0 {
		return writeBatchImageProviderFailureLine(w, customID, "EMPTY_IMAGE_OUTPUT", "provider response contained no image output")
	}
	parts := make([]map[string]any, 0, len(images))
	for _, image := range images {
		mimeType := strings.TrimSpace(image.MimeType)
		if mimeType == "" {
			mimeType = detectImageContentType(image.Data)
		}
		if len(image.Data) == 0 {
			continue
		}
		parts = append(parts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": mimeType,
				"data":     base64.StdEncoding.EncodeToString(image.Data),
			},
		})
	}
	if len(parts) == 0 {
		return writeBatchImageProviderFailureLine(w, customID, "EMPTY_IMAGE_OUTPUT", "provider response contained no image output")
	}
	line := map[string]any{
		"key": customID,
		"response": map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{"parts": parts},
			}},
		},
	}
	return json.NewEncoder(w).Encode(line)
}

func writeBatchImageProviderFailureLine(w io.Writer, customID, code, message string) error {
	line := map[string]any{
		"key": customID,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	return json.NewEncoder(w).Encode(line)
}

func extractProviderImageResults(ctx context.Context, client *http.Client, payload any) ([]providerImageResult, error) {
	var out []providerImageResult
	collectProviderImages(payload, &out)
	for i := range out {
		if len(out[i].Data) == 0 && strings.TrimSpace(out[i].MimeType) == "" {
			continue
		}
		if out[i].MimeType == "" {
			out[i].MimeType = detectImageContentType(out[i].Data)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	for _, rawURL := range collectProviderImageURLs(payload) {
		image, err := downloadProviderImage(ctx, client, rawURL)
		if err != nil {
			return nil, err
		}
		out = append(out, image)
	}
	if len(out) == 0 {
		return nil, batchImageProviderInputError("provider response contained no image output")
	}
	return out, nil
}

func collectProviderImages(node any, out *[]providerImageResult) {
	switch v := node.(type) {
	case map[string]any:
		if image := providerImageFromMap(v); len(image.Data) > 0 {
			*out = append(*out, image)
		}
		for _, child := range v {
			collectProviderImages(child, out)
		}
	case []any:
		for _, child := range v {
			collectProviderImages(child, out)
		}
	}
}

func providerImageFromMap(m map[string]any) providerImageResult {
	for _, key := range []string{"b64_json", "base64", "image_base64", "imageBase64"} {
		if raw, ok := m[key].(string); ok {
			if data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw)); err == nil && len(data) > 0 {
				return providerImageResult{MimeType: firstProviderImageMime(m), Data: data}
			}
		}
	}
	for _, key := range []string{"inlineData", "inline_data"} {
		inline, ok := m[key].(map[string]any)
		if !ok {
			continue
		}
		raw, _ := inline["data"].(string)
		if data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw)); err == nil && len(data) > 0 {
			mimeType := firstNonEmptyString(inline["mimeType"], inline["mime_type"], m["mime_type"], m["content_type"])
			return providerImageResult{MimeType: mimeType, Data: data}
		}
	}
	return providerImageResult{}
}

func collectProviderImageURLs(node any) []string {
	seen := make(map[string]struct{})
	var out []string
	var walk func(any)
	walk = func(current any) {
		switch v := current.(type) {
		case map[string]any:
			for _, key := range []string{"url", "image_url", "download_url"} {
				if raw, ok := v[key].(string); ok {
					raw = strings.TrimSpace(raw)
					if raw == "" || !looksLikeImageURL(raw) {
						continue
					}
					if _, exists := seen[raw]; exists {
						continue
					}
					seen[raw] = struct{}{}
					out = append(out, raw)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		case string:
			raw := strings.TrimSpace(v)
			if raw == "" || !looksLikeImageURL(raw) {
				return
			}
			if _, exists := seen[raw]; exists {
				return
			}
			seen[raw] = struct{}{}
			out = append(out, raw)
		}
	}
	walk(node)
	return out
}

func looksLikeImageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return false
	}
	if strings.EqualFold(u.Scheme, "data") {
		return strings.HasPrefix(strings.ToLower(raw), "data:image/")
	}
	return strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")
}

func downloadProviderImage(ctx context.Context, client *http.Client, rawURL string) (providerImageResult, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "data:image/") {
		data, mimeType, err := decodeBatchImageDataURL(rawURL)
		return providerImageResult{MimeType: mimeType, Data: data}, err
	}
	if client == nil {
		client = batchImageDefaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return providerImageResult{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return providerImageResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return providerImageResult{}, fmt.Errorf("download image status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, defaultImageMaxDownloadBytes+1))
	if err != nil {
		return providerImageResult{}, err
	}
	if int64(len(data)) > defaultImageMaxDownloadBytes {
		return providerImageResult{}, fmt.Errorf("downloaded image exceeds %d bytes", defaultImageMaxDownloadBytes)
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		mimeType = detectImageContentType(data)
	}
	return providerImageResult{MimeType: mimeType, Data: data}, nil
}

func decodeBatchImageDataURL(raw string) ([]byte, string, error) {
	header, payload, ok := strings.Cut(strings.TrimSpace(raw), ",")
	if !ok {
		return nil, "", batchImageProviderInputError("invalid image data URL")
	}
	mimeType := strings.TrimPrefix(strings.Split(strings.TrimPrefix(header, "data:"), ";")[0], "data:")
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return nil, "", batchImageProviderInputError("data URL is not an image")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", err
	}
	return data, mimeType, nil
}

func firstProviderImageMime(m map[string]any) string {
	return firstNonEmptyString(m["mime_type"], m["mimeType"], m["content_type"], m["contentType"])
}

func batchImageOpenAIOutputSize(aspectRatio string) string {
	switch strings.TrimSpace(aspectRatio) {
	case "16:9":
		return "1536x1024"
	case "9:16", "4:5", "3:4":
		return "1024x1536"
	default:
		return "1024x1024"
	}
}

func batchImageMiniMaxSize(aspectRatio string) string {
	switch strings.TrimSpace(aspectRatio) {
	case "16:9":
		return "16:9"
	case "9:16":
		return "9:16"
	case "4:5":
		return "4:5"
	case "3:4":
		return "3:4"
	default:
		return "1:1"
	}
}

func batchImageOpenAIOutputFormat(mimeType string) string {
	switch normalizeBatchImageReferenceMimeType(mimeType) {
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	case "image/png":
		return "png"
	default:
		return ""
	}
}

func batchImageFileExtensionWithDot(mimeType string) string {
	ext := batchImageFileExtension(mimeType)
	if ext == "" {
		return ".png"
	}
	return "." + ext
}

func minimaxBaseResponseError(resp map[string]any) error {
	if resp == nil {
		return batchImageProviderInputError("empty MiniMax response")
	}
	base, _ := resp["base_resp"].(map[string]any)
	if base == nil {
		base, _ = resp["baseResp"].(map[string]any)
	}
	if base == nil {
		return nil
	}
	code := batchImageProviderScalarString(base["status_code"], base["statusCode"], base["code"])
	if code == "" || code == "0" {
		return nil
	}
	msg := firstNonEmptyString(base["status_msg"], base["statusMsg"], base["message"], base["msg"])
	if msg == "" {
		msg = "MiniMax API request failed"
	}
	return fmt.Errorf("MiniMax status %s: %s", code, msg)
}

func openAIImageProviderError(reason, message string, cause error) error {
	err := infraerrors.New(http.StatusBadGateway, reason, message)
	if cause != nil {
		return err.WithCause(cause)
	}
	return err
}

func batchImageProviderScalarString(values ...any) string {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case float64:
			return strconv.FormatInt(int64(v), 10)
		case int:
			return strconv.Itoa(v)
		case int64:
			return strconv.FormatInt(v, 10)
		case json.Number:
			return v.String()
		}
	}
	return ""
}

func minimaxImageProviderError(reason, message string, cause error) error {
	err := infraerrors.New(http.StatusBadGateway, reason, message)
	if cause != nil {
		return err.WithCause(cause)
	}
	return err
}
