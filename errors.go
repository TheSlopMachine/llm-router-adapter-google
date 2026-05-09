package google

import (
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/TheSlopMachine/llm-router-sdk"
)

func parseGoogleError(statusCode int, body []byte) error {
	var googleErr GoogleErrorResponse
	errorMsg := string(body)

	if err := json.Unmarshal(body, &googleErr); err == nil && googleErr.Error.Message != "" {
		errorMsg = googleErr.Error.Message
	}

	switch statusCode {
	case 429:
		retryAfter := time.Now().Add(60 * time.Second)
		return &sdk.ProviderError{
			StatusCode: 429,
			Message:    errorMsg,
			Type:       sdk.ErrorTypeRateLimit,
			RetryAfter: &retryAfter,
		}
	case 401, 403:
		return &sdk.ProviderError{
			StatusCode: statusCode,
			Message:    errorMsg,
			Type:       sdk.ErrorTypeAuth,
		}
	case 500, 502, 503:
		return &sdk.ProviderError{
			StatusCode: statusCode,
			Message:    errorMsg,
			Type:       sdk.ErrorTypeUpstream,
		}
	case 504:
		return &sdk.ProviderError{
			StatusCode: 504,
			Message:    errorMsg,
			Type:       sdk.ErrorTypeTimeout,
		}
	default:
		return fmt.Errorf("google api error (%d): %s", statusCode, errorMsg)
	}
}
