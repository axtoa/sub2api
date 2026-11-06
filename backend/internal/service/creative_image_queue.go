package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const creativeImageRequestTimeout = 5 * time.Minute

// Persist the request on the same data volume as image outputs. The existing
// Redis worker owns generation; no upstream request is tied to the browser.
func submitCreativeImageInput(ctx context.Context, root, provider string, input BatchImageInput) (*BatchProviderJob, error) {
	ref, err := writeBatchImageLocalResultMode(ctx, root, provider, input.BatchID+"-input", 0o600, func(w io.Writer) error { return json.NewEncoder(w).Encode(input) })
	if err != nil {
		return nil, err
	}
	return &BatchProviderJob{ProviderJobName: input.BatchID, ProviderInputRef: ref, RawState: string(BatchProviderStateQueued)}, nil
}

type creativeImageGenerate func(context.Context, *batchImageHTTPClient, BatchImageInput, BatchImageInputItem) ([]providerImageResult, error)

func processCreativeImageInput(ctx context.Context, root, provider string, job *BatchImageJob, account *Account, httpClient *http.Client, generate creativeImageGenerate) (*BatchProviderStatus, error) {
	inputPath, err := batchImageLocalResultPath(root, *job.ProviderInputRef)
	if err != nil {
		return nil, err
	}
	outputRef := fmt.Sprintf("local-jsonl:%s/%s.jsonl", provider, job.BatchID)
	outputPath, err := batchImageLocalResultPath(root, outputRef)
	if err != nil {
		return nil, err
	}
	complete := func() *BatchProviderStatus {
		return &BatchProviderStatus{RawState: string(BatchProviderStateSucceeded), InternalState: BatchProviderStateSucceeded, Done: true, ProviderOutputRef: outputRef}
	}
	if _, err := os.Stat(outputPath); err == nil {
		return complete(), nil
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}
	var input BatchImageInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	accountClient, err := creativeMediaAccountClient(account, httpClient)
	if err != nil {
		return nil, err
	}
	client := newBatchImageHTTPClient(account.GetOpenAIBaseURL(), batchImageProviderAPIKey(account), accountClient)
	checkpointDir := inputPath + ".parts"
	if err := os.MkdirAll(checkpointDir, 0700); err != nil {
		return nil, err
	}
	for i, item := range input.Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		checkpoint := filepath.Join(checkpointDir, fmt.Sprintf("%06d.jsonl", i))
		if _, err := os.Stat(checkpoint); err == nil {
			continue
		}
		marker, markerErr := os.OpenFile(checkpoint+".started", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		var line bytes.Buffer
		if _, cancelErr := os.Stat(inputPath + ".cancelled"); cancelErr == nil && markerErr == nil {
			if marker != nil {
				_ = marker.Close()
			}
			err = writeBatchImageProviderFailureLine(&line, item.CustomID, "CANCELLED", "已取消")
		} else if errors.Is(markerErr, os.ErrExist) {
			// A second worker may arrive while the first is still generating (for
			// example after a Redis lease expires). Do not overwrite its result.
			info, statErr := os.Stat(checkpoint + ".started")
			if statErr != nil {
				return nil, statErr
			}
			if time.Since(info.ModTime()) < creativeImageRequestTimeout+time.Minute {
				return &BatchProviderStatus{InternalState: BatchProviderStateRunning, SuggestedRequeueAfter: 10 * time.Second}, nil
			}
			// A crash after sending cannot be retried safely without provider
			// idempotency. Preserve known successes and surface this one for review.
			err = writeBatchImageProviderFailureLine(&line, item.CustomID, "UPSTREAM_RESULT_UNCERTAIN", "生成中断，结果待核对；未自动重复请求")
		} else {
			if markerErr != nil {
				return nil, markerErr
			}
			if err := marker.Close(); err != nil {
				return nil, err
			}
			itemCtx, cancel := context.WithTimeout(ctx, creativeImageRequestTimeout)
			images, generateErr := generate(itemCtx, client, input, item)
			cancel()
			if generateErr != nil {
				err = writeBatchImageProviderFailureLine(&line, item.CustomID, "IMAGE_GENERATION_FAILED", sanitizeBatchImagePublicMessage(generateErr.Error()))
			} else {
				err = writeBatchImageProviderSuccessLine(&line, item.CustomID, images)
			}
		}
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(checkpoint+".tmp", line.Bytes(), 0600); err != nil {
			return nil, err
		}
		if err := os.Rename(checkpoint+".tmp", checkpoint); err != nil {
			return nil, err
		}
		if i < len(input.Items)-1 {
			return &BatchProviderStatus{InternalState: BatchProviderStateRunning, SuggestedRequeueAfter: time.Second}, nil
		}
	}
	_, err = writeBatchImageLocalResult(ctx, root, provider, job.BatchID, func(w io.Writer) error {
		for i := range input.Items {
			data, err := os.ReadFile(filepath.Join(checkpointDir, fmt.Sprintf("%06d.jsonl", i)))
			if err != nil {
				return err
			}
			if _, err := w.Write(data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return complete(), nil
}

func cleanupCreativeImageInput(root string, job *BatchImageJob) error {
	if job.ProviderInputRef == nil {
		return nil
	}
	path, err := batchImageLocalResultPath(root, *job.ProviderInputRef)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path + ".parts"); err != nil {
		return err
	}
	if err := os.Remove(path + ".cancelled"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return removeBatchImageLocalResult(root, *job.ProviderInputRef)
}

func cancelCreativeImageInput(root string, job *BatchImageJob) error {
	if job.ProviderInputRef == nil {
		return nil
	}
	path, err := batchImageLocalResultPath(root, *job.ProviderInputRef)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".cancelled", nil, 0600)
}
