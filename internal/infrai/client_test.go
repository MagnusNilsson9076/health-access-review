package infrai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestListKeysDecodesBusinessRejectionBeforeStatus(t *testing.T) {
	client := New("test-key")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", req.Method)
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":false,"error":{"code":"review-policy","message":"review policy rejected the request"}}`)),
		}, nil
	})

	_, err := client.ListKeys(context.Background())
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Code != "review-policy" {
		t.Fatalf("API error = %#v", apiErr)
	}
}

func TestListKeysHonorsRetryAfter(t *testing.T) {
	attempts := 0
	var slept time.Duration
	client := New("test-key")
	client.sleep = func(_ context.Context, delay time.Duration) error { slept = delay; return nil }
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"2"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":false,"error":{"code":"rate-limit","message":"retry later"}}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"data":{"keys":[]},"metadata":{}}`)),
		}, nil
	})

	if _, err := client.ListKeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || slept != 2*time.Second {
		t.Fatalf("attempts = %d, slept = %s", attempts, slept)
	}
}
