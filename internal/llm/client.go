package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("openrouter error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp openRouterResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse openrouter response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", 0, fmt.Errorf("no response from llm")
	}

	return apiResp.Choices[0].Message.Content, apiResp.Usage.Cost, nil
}
