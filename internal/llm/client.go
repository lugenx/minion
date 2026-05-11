package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type openRouterRequest struct {
	Model          string                 `json:"model"`
	Messages       []message              `json:"messages"`
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Usage struct {
		Cost float64 `json:"cost"`
	} `json:"usage"`
}

// Custom client to interact with OpenRouter
func performChatCompletion(ctx context.Context, model string, sysPrompt string, userPrompt string) (string, float64, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", 0, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}

	reqBody := openRouterRequest{
		Model: model,
		Messages: []message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: map[string]interface{}{
			"type": "json_object",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	maxRetries := 4
	baseDelay := 2 * time.Second
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Recreate the request for each attempt because the body buffer is consumed
		req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return "", 0, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{
			Timeout: 15 * time.Second,
		}
		resp, err := client.Do(req)

		var retryable bool

		if err != nil {
			lastErr = fmt.Errorf("http request failed: %w", err)
			retryable = true // Network errors are retryable
		} else {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()

			if readErr != nil {
				lastErr = fmt.Errorf("failed to read response body: %w", readErr)
				retryable = true
			} else if resp.StatusCode == http.StatusOK {
				// Success!
				var apiResp openRouterResponse
				if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
					return "", 0, fmt.Errorf("failed to parse openrouter response: %w", err)
				}

				if len(apiResp.Choices) == 0 {
					return "", 0, fmt.Errorf("no response from llm")
				}

				return apiResp.Choices[0].Message.Content, apiResp.Usage.Cost, nil
			} else if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("openrouter error (status %d): %s", resp.StatusCode, string(bodyBytes))
				retryable = true
			} else {
				// Non-retryable HTTP error (e.g., 400 Bad Request, 401 Unauthorized)
				return "", 0, fmt.Errorf("openrouter error (status %d): %s", resp.StatusCode, string(bodyBytes))
			}
		}

		if retryable && attempt < maxRetries {
			delay := baseDelay * time.Duration(1<<attempt) // Exponential backoff: 2s, 4s, 8s, 16s
			select {
			case <-time.After(delay):
				// Backoff complete, continue to next attempt
			case <-ctx.Done():
				return "", 0, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			}
		}
	}

	return "", 0, fmt.Errorf("max retries exhausted. last error: %w", lastErr)
}
