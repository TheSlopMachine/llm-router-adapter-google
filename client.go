package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func (c *Client) generateContent(ctx context.Context, apiKey, model string, req *GenerateContentRequest) (*GenerateContentResponse, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, model)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, parseGoogleError(resp.StatusCode, respBody)
	}

	var googleResp GenerateContentResponse
	if err := json.Unmarshal(respBody, &googleResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &googleResp, nil
}

func (c *Client) streamGenerateContent(ctx context.Context, apiKey, model string, req *GenerateContentRequest) (*http.Response, error) {
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent", c.baseURL, model)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func newClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}
