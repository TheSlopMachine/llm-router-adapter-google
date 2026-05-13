package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

func (c *Client) listModels(ctx context.Context, apiKey string) ([]ModelMetadata, error) {
	allModels := []ModelMetadata{}
	pageToken := ""

	for {
		url := fmt.Sprintf("%s/models", c.baseURL)
		if pageToken != "" {
			url += "?pageToken=" + pageToken
		}

		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("x-goog-api-key", apiKey)

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

		var listResp ListModelsResponse
		if err := json.Unmarshal(respBody, &listResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		allModels = append(allModels, listResp.Models...)

		if listResp.NextPageToken == "" {
			break
		}
		pageToken = listResp.NextPageToken
	}

	return allModels, nil
}

func (c *Client) extractRateLimits(ctx context.Context, apiKey, model string) (rpm, tpm, rpd int64, err error) {
	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, model)

	testReq := &GenerateContentRequest{
		Contents: []Content{
			{
				Parts: []Part{{Text: "ping"}},
			},
		},
	}

	body, err := json.Marshal(testReq)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-goog-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	io.ReadAll(resp.Body)

	rateLimitHeaders := make(map[string]string)
	for key, values := range resp.Header {
		keyLower := strings.ToLower(key)
		if strings.HasPrefix(keyLower, "x-ratelimit") {
			if len(values) > 0 {
				rateLimitHeaders[keyLower] = values[0]
			}
		}
	}

	rpm = parseRateLimitValue(rateLimitHeaders["x-ratelimit-limit-requests-per-minute"])
	if rpm == 0 {
		rpm = parseRateLimitValue(rateLimitHeaders["x-ratelimit-requests-per-minute"])
	}
	if rpm == 0 {
		rpm = parseRateLimitValue(rateLimitHeaders["x-ratelimit-limit"])
	}

	tpm = parseRateLimitValue(rateLimitHeaders["x-ratelimit-limit-tokens-per-minute"])
	if tpm == 0 {
		tpm = parseRateLimitValue(rateLimitHeaders["x-ratelimit-tokens-per-minute"])
	}

	rpd = parseRateLimitValue(rateLimitHeaders["x-ratelimit-limit-requests-per-day"])
	if rpd == 0 {
		rpd = parseRateLimitValue(rateLimitHeaders["x-ratelimit-requests-per-day"])
	}

	return rpm, tpm, rpd, nil
}

func parseRateLimitValue(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func newClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}
