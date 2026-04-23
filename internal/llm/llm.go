package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Match represents a single item found in the text that matches the criteria.
type Match struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

// EvalResult represents the JSON output expected from the LLM.
type EvalResult struct {
	Matches []Match `json:"matches"`
}

// Evaluator wraps the OpenAI client configured for OpenRouter.
type Evaluator struct {
	client *openai.Client
	model  string
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator() (*Evaluator, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}

	model := os.Getenv("DEFAULT_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	client := openai.NewClientWithConfig(config)

	return &Evaluator{
		client: client,
		model:  model,
	}, nil
}

// EvaluateText asks the LLM to evaluate the text against the provided rules.
func (e *Evaluator) EvaluateText(ctx context.Context, text string, task string) (*EvalResult, error) {
	currentDate := time.Now().Format("Monday, January 2, 2006 at 15:04 MST")
	
	systemPrompt := "You are an autonomous extraction engine. Your job is to read the provided text and fulfill the user's task.\n\n"
	
	systemPrompt += fmt.Sprintf("CRITICAL TEMPORAL CONTEXT:\nToday's date and time is %s. Use this as your reference point for any time-based rules in the user's task.\n\n", currentDate)
	
	systemPrompt += "--- USER TASK START ---\n"
	if task != "" {
		systemPrompt += task + "\n"
	} else {
		systemPrompt += "Extract all relevant information from the text.\n"
	}
	systemPrompt += "--- USER TASK END ---\n\n"
	
	systemPrompt += "MECHANICAL RULES:\n"
	systemPrompt += "- Extract ALL independent items from the text that fulfill the user's task.\n"
	systemPrompt += "- If the text provides a specific [Link: URL] for the item, extract it into the 'url' field. Otherwise, leave it blank.\n"
	systemPrompt += "- If no items match the task, return an empty array for matches.\n"
	systemPrompt += "- You MUST output ONLY a valid JSON object matching this schema exactly:\n"
	systemPrompt += `{
  "matches": [
    {
      "title": "Name or title of the matched item/event",
      "url": "Specific url for this event if found, else empty",
      "summary": "1 sentence explanation of what it is and why it passed. (Note: This summary will be sent directly to the user, so write it for them to read)."
    }
  ]
}`

	req := openai.ChatCompletionRequest{
		Model: e.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	resp, err := e.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from llm")
	}

	content := resp.Choices[0].Message.Content

	var result EvalResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse llm json output: %w. raw output: %s", err, content)
	}

	return &result, nil
}