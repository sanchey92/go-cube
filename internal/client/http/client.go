package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPClient struct {
	client  *http.Client
	timeout time.Duration
}

func NewHTTPClient(timeout time.Duration) *HTTPClient {
	if timeout == 0 {
		timeout = time.Second * 10
	}

	return &HTTPClient{
		client:  &http.Client{Timeout: timeout},
		timeout: timeout,
	}
}

func (c *HTTPClient) Get(ctx context.Context, url string, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, url, nil, result)
}

func (c *HTTPClient) Post(ctx context.Context, url string, body io.Reader, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, url, body, result)
}

func (c *HTTPClient) Put(ctx context.Context, url string, body io.Reader, result interface{}) error {
	return c.doRequest(ctx, http.MethodPut, url, body, result)
}

func (c *HTTPClient) Delete(ctx context.Context, url string) error {
	return c.doRequest(ctx, http.MethodDelete, url, nil, nil)
}

func (c *HTTPClient) doRequest(ctx context.Context, method, url string, body io.Reader, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
