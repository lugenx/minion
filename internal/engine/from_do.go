package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"minion/internal/config"
	"minion/internal/llm"
	"minion/internal/types"
)

type doMatch struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

type doResult struct {
	Matches []doMatch `json:"matches"`
}

func processDoOnly(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	if minion.Do == "" {
		return nil
	}

	step("do", "analyzing with do prompt (no source URLs)", false)
	runCtx.Stats.Analyzed++

	model := minion.Settings.Model
	if model == "" {
		model = os.Getenv("DEFAULT_MODEL")
	}
	if model == "" {
		runCtx.Stats.Errors++
		step("do", "no model configured: set DEFAULT_MODEL in .env or add model in settings", true)
		return nil
	}

	currentDate := time.Now().Format("Monday, January 2, 2006 at 15:04 MST")

	systemPrompt := "You are a helpful assistant. Your job is to fulfill the user's task using your own knowledge.\n\n"
	systemPrompt += fmt.Sprintf("CRITICAL TEMPORAL CONTEXT:\nToday's date and time is %s. Use this as your reference point for any time-based rules.\n\n", currentDate)
	systemPrompt += "--- USER TASK START ---\n"
	systemPrompt += minion.Do + "\n"
	systemPrompt += "--- USER TASK END ---\n\n"
	systemPrompt += "MECHANICAL RULES:\n"
	systemPrompt += "- Provide ALL relevant items based on the user's task.\n"
	systemPrompt += "- If an item has a specific URL, include it in the 'url' field. Otherwise, leave it blank.\n"
	systemPrompt += "- If no items match the task, return an empty array for matches.\n"
	systemPrompt += "- You MUST output ONLY a valid JSON object matching this schema exactly:\n"
	systemPrompt += `{
  "matches": [
    {
      "title": "This is the name or title of the matched item. Look at the USER TASK to see anything you need to modify or incorporate to this field to fulfill the instructions. Default to the exact name or title of the item, but prioritize the USER TASK if there is any conflict.",
      "url": "This is the specific URL for the matched item. Look at the USER TASK to see anything you need to modify or incorporate to this field to fulfill the instructions. Default to the exact URL found in the text (leave empty if none), but prioritize the USER TASK if there is any conflict.",
      "summary": "This is the summary of the matched item. Look at the USER TASK to see anything you need to add or incorporate to this field to fulfill the instructions. Default to a 1-sentence explanation of what the item is and why it matched, but prioritize the USER TASK if there is any conflict. (Note: This is sent directly to the user, so write it for them to read using 'you' and 'your')."
    }
  ]
}`

	userMessage := minion.Do

	evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
	raw, cost, err := llm.Chat(evalCtx, model, systemPrompt, userMessage, true)
	evalCancel()

	if err != nil {
		runCtx.Stats.Errors++
		step("do", fmt.Sprintf("→ %v", err), true)
		return nil
	}

	runCtx.Stats.TotalCost += cost

	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
	}
	if strings.HasSuffix(raw, "```") {
		raw = strings.TrimSuffix(raw, "```")
	}

	startIdx := strings.Index(raw, "{")
	endIdx := strings.LastIndex(raw, "}")
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		raw = raw[startIdx : endIdx+1]
	}
	raw = strings.TrimSpace(raw)

	var res doResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		runCtx.Stats.Errors++
		step("do", fmt.Sprintf("failed to parse llm json output: %v", err), true)
		return nil
	}

	nextArray := buildDoResultItems(res.Matches)

	if len(res.Matches) == 0 {
		runCtx.Stats.Skipped++
		step("skip", "no matches", false)
	}

	runCtx.Stats.Results += len(res.Matches)

	if len(nextArray) == 0 {
		return nil
	}

	if len(minion.Tell) > 0 {
		deliverTargets(ctx, minion, runCtx, nextArray, minion.Tell, true)
	}

	return nil
}

func buildDoResultItems(matches []doMatch) []types.Item {
	var items []types.Item
	for _, aiMatch := range matches {
		itemURL := strings.TrimSuffix(aiMatch.URL, "/")
		items = append(items, types.Item{
			ID:      generateID(),
			URL:     itemURL,
			Title:   aiMatch.Title,
			Summary: aiMatch.Summary,
		})
	}
	return items
}
