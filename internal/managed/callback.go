package managed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultRuntimeMaxAttempts = 3

type runtimeSignedCallbackOptions struct {
	CallbackURL string
	Secret      []byte
	Event       string
	Now         time.Time
	Client      *http.Client
	MaxAttempts int
	RetryDelay  time.Duration
	Payload     any
}

type runtimeSignedCallbackResult struct {
	StatusCode int
}

func postSignedRuntimeCallback(ctx context.Context, opts runtimeSignedCallbackOptions) (runtimeSignedCallbackResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	callbackURL, err := validateRuntimeCallbackURL(opts.CallbackURL)
	if err != nil {
		return runtimeSignedCallbackResult{}, err
	}
	if err := validateBootstrapSecret(opts.Secret); err != nil {
		return runtimeSignedCallbackResult{}, fmt.Errorf("runtime callback secret is too short")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	body, err := json.Marshal(opts.Payload)
	if err != nil {
		return runtimeSignedCallbackResult{}, fmt.Errorf("marshal runtime callback payload: %w", err)
	}
	timestamp := now.UTC().Format(time.RFC3339)
	signature := signRuntimeCallback(opts.Event, timestamp, body, opts.Secret)
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: defaultRuntimeHTTPTimeout}
	}
	attempts := opts.MaxAttempts
	if attempts <= 0 {
		attempts = defaultRuntimeMaxAttempts
	}
	retryDelay := opts.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 100 * time.Millisecond
	}

	var lastStatus int
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
		if err != nil {
			return runtimeSignedCallbackResult{}, fmt.Errorf("create runtime callback request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set(RuntimeEventHeader, opts.Event)
		req.Header.Set(RuntimeTimestampHeader, timestamp)
		req.Header.Set(RuntimeSignatureHeader, runtimeSignaturePrefix+signature)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < attempts && ctx.Err() == nil {
				waitRuntimeRetry(ctx, retryDelay)
				continue
			}
			return runtimeSignedCallbackResult{}, fmt.Errorf("post runtime callback: %w", err)
		}
		lastStatus = resp.StatusCode
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			return runtimeSignedCallbackResult{StatusCode: resp.StatusCode}, nil
		}
		if !retryableRuntimeStatus(resp.StatusCode) || attempt == attempts {
			return runtimeSignedCallbackResult{StatusCode: resp.StatusCode}, fmt.Errorf("runtime callback failed with status %d", resp.StatusCode)
		}
		waitRuntimeRetry(ctx, retryDelay)
	}
	if lastErr != nil {
		return runtimeSignedCallbackResult{StatusCode: lastStatus}, fmt.Errorf("post runtime callback: %w", lastErr)
	}
	return runtimeSignedCallbackResult{StatusCode: lastStatus}, fmt.Errorf("runtime callback failed")
}

func retryableRuntimeStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func waitRuntimeRetry(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
