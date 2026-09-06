package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type structuredChatFunc func(
	ctx context.Context,
	model string,
	systemPrompt string,
	userPrompt string,
	jsonMode bool,
) (string, float64, error)

func requestStructured[T any](
	ctx context.Context,
	model string,
	systemPrompt string,
	userPrompt string,
	chat structuredChatFunc,
) (T, float64, error) {
	var zero T
	var totalCost float64
	prompt := systemPrompt

	for attempt := 0; attempt < 2; attempt++ {
		raw, cost, err := chat(ctx, model, prompt, userPrompt, true)
		totalCost += cost
		if err != nil {
			return zero, totalCost, err
		}

		var result T
		if err := json.Unmarshal([]byte(extractJSONObject(raw)), &result); err == nil {
			return result, totalCost, nil
		} else if attempt == 0 {
			prompt = systemPrompt + fmt.Sprintf(
				"\n\nRETRY CORRECTION:\nYour previous response could not be decoded into the required schema: %v\nReturn only one JSON object matching the schema exactly. Keep the original task and record unchanged.",
				err,
			)
		} else {
			return zero, totalCost, fmt.Errorf("failed to parse llm output after repair retry: %w", err)
		}
	}

	return zero, totalCost, fmt.Errorf("structured response retry exhausted")
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
	}
	if strings.HasSuffix(raw, "```") {
		raw = strings.TrimSuffix(raw, "```")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end >= start {
		raw = raw[start : end+1]
	}
	return strings.TrimSpace(raw)
}
