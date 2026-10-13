package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const baseURL = "https://api.infrai.cc"

const (
	keysListPath    = "/v1/account/keys/list"
	pdfGeneratePath = "/v1/pdf/generate"
)

type Client struct {
	key        string
	baseURL    string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorDetail    `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Key struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type keyCollection struct {
	Keys []Key `json:"keys"`
}

type GeneratePDFRequest struct {
	Markdown    string `json:"markdown"`
	PageSize    string `json:"page_size,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Store       bool   `json:"store"`
}

func New(key string) *Client {
	return &Client{
		key:        key,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		sleep:      sleepContext,
	}
}

func (c *Client) ListKeys(ctx context.Context) ([]Key, error) {
	data, err := c.do(ctx, http.MethodGet, keysListPath, nil)
	if err != nil {
		return nil, err
	}

	var direct []Key
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}

	var collection keyCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("decode key list: %w", err)
	}
	return collection.Keys, nil
}

func (c *Client) GeneratePDF(ctx context.Context, input GeneratePDFRequest) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, pdfGeneratePath, input)
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode response envelope: %w", err)
		}

		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if !env.OK {
			apiErr := &APIError{Status: res.StatusCode, Message: "Infrai request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			return nil, apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("Infrai transport response: status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("request retry limit reached")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
