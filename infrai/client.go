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
	"strings"
	"time"
)

const errorsCapturePath = "/v1/errors/capture"

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type CaptureInput struct {
	Message     string         `json:"message"`
	Level       string         `json:"level"`
	Fingerprint []string       `json:"fingerprint"`
	Exception   string         `json:"exception"`
	Context     map[string]any `json:"context"`
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string { return fmt.Sprintf("infrai %s: %s", e.Code, e.Message) }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func New(baseURL, key string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: httpClient, sleep: sleepContext}
}

// Capture implements the infrai.errors.capture calling pattern.
func (c *Client) Capture(ctx context.Context, eventID string, input CaptureInput) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode capture: %w", err)
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+errorsCapturePath, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create capture request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", eventID)

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send capture: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read capture response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			apiErr := envelopeError(env.Error, res.StatusCode)
			if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
				return apiErr
			}
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode capture envelope: %w", decodeErr)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("capture transport status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("capture retry budget exhausted")
}

func envelopeError(body *errorBody, status int) error {
	if body == nil {
		return &APIError{Code: "request_rejected", Message: "request rejected", HTTPStatus: status}
	}
	message := body.Message
	if message == "" {
		message = body.Hint
	}
	return &APIError{Code: body.Code, Message: message, HTTPStatus: status}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
