package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
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

// Evaluator wraps the LLM client.
type Evaluator struct {
	model string
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator() (*Evaluator, error) {
	model := os.Getenv("DEFAULT_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}

	return &Evaluator{
		model: model,
	}, nil
}

// EvaluateText asks the LLM to evaluate the text against the provided rules.
func (e *Evaluator) EvaluateText(ctx context.Context, text string, task string, format string) (*EvalResult, float64, error) {
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
	systemPrompt += "- CACHING: Return 'discard' if the page is permanently off-topic, junk, or old. Return 'skip' for lists that update frequently, or for relevant pages that simply fail a dynamic rule (like how \"next 5 days\" or prices change every day) so we can check them again later.\n"
	systemPrompt += "- You MUST output ONLY a valid JSON object matching this schema exactly:\n"
	systemPrompt += `{
  "cache_action": "discard" | "skip",
  "matches": [
    {
      "title": "This is the name or title of the matched item. Look at the USER TASK to see anything you need to modify or incorporate to this field to fulfill the instructions. Default to the exact name or title of the item, but prioritize the USER TASK if there is any conflict.",
      "url": "This is the specific URL for the matched item. Look at the USER TASK to see anything you need to modify or incorporate to this field to fulfill the instructions. Default to the exact URL found in the text (leave empty if none), but prioritize the USER TASK if there is any conflict.",
      "summary": "This is the summary of the matched item. Look at the USER TASK to see anything you need to add or incorporate to this field to fulfill the instructions. Default to a 1-sentence explanation of what the item is and why it matched, but prioritize the USER TASK if there is any conflict. (Note: This is sent directly to the user, so write it for them to read using 'you' and 'your')."
    }
  ]
}`

	content, cost, err := performChatCompletion(ctx, e.model, systemPrompt, text)
	if err != nil {
		return nil, 0, err
	}

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
		return nil, cost, fmt.Errorf("failed to parse llm json output: %w. raw output: %s", err, content)
	}

	return &result, cost, nil
}
