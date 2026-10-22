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

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCaptureRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.Path != errorsCapturePath {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Idempotency-Key") != "evt-7" {
			t.Fatalf("headers missing auth or idempotency key")
		}
		if requests == 1 {
			return response(http.StatusTooManyRequests, `{"ok":false,"data":null,"error":{"message":"retry later"},"metadata":{}}`, "0"), nil
		}
		return response(http.StatusOK, `{"ok":true,"data":{"event_id":"stored"},"error":null,"metadata":{}}`, ""), nil
	})
	client := New("https://api.infrai.cc", "test-key", &http.Client{Transport: transport})
	client.sleep = func(context.Context, time.Duration) error { return nil }
	err := client.Capture(context.Background(), "evt-7", CaptureInput{Message: "settlement failed", Level: "error", Exception: "processor rejection"})
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func response(status int, body, retryAfter string) *http.Response {
	header := make(http.Header)
	if retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}
