package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	CacheAction string  `json:"cache_action"`
	Matches     []Match `json:"matches"`
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
func (e *Evaluator) EvaluateText(ctx context.Context, text string, task string, format string) (*EvalResult, error) {
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
	systemPrompt += "- CACHING: Return 'permanent_drop' if the page is off-topic, junk, or old. Return 're_evaluate_later' for lists that update frequently, or for relevant pages that simply fail a dynamic rule (like how \"next 5 days\" or prices change every day) so we can check them again later.\n"
	systemPrompt += "- You MUST output ONLY a valid JSON object matching this schema exactly:\n"
	systemPrompt += `{
  "cache_action": "permanent_drop" | "re_evaluate_later",
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

	// Strip Markdown formatting if the AI added it (e.g. ```json ... ```)
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}
	
	// Extract only the JSON block (from first { to last })
	// This prevents "Chain of Thought" conversational text from breaking the parser
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		content = content[startIdx : endIdx+1]
	}

	content = strings.TrimSpace(content)

	var result EvalResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse llm json output: %w. raw output: %s", err, content)
	}

	return &result, nil
}