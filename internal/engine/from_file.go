package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"minion/internal/config"
	"minion/internal/llm"
	"minion/internal/types"
)

type fileMatch struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

type fileResult struct {
	Matches []fileMatch `json:"matches"`
}

func processFileItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	filePath := strictExpandEnv(item.FilePath)
	if filePath == "" {
		return nil
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		step("from", fmt.Sprintf("file not found: `%s`", filePath), false)
		return nil
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := strings.TrimSpace(string(raw))
	if content == "" {
		step("from", fmt.Sprintf("empty file: `%s`", filePath), false)
		return nil
	}

	parts := splitDocs(content)

	cursorHash := ""
	if !runCtx.Ephemeral {
		cursorHash, err = runCtx.Store.GetFileHash(filePath, minion.Filename)
		if err != nil {
			step("from", fmt.Sprintf("cursor read error: %v", err), true)
		}
	}

	docStart := 0
	if cursorHash != "" {
		for i, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			rawHash := sha256.Sum256([]byte(trimmed))
			h := fmt.Sprintf("%x", rawHash[:8])
			if h == cursorHash {
				docStart = i + 1
				break
			}
		}
	}

	var docs []string
	for i := docStart; i < len(parts); i++ {
		t := strings.TrimSpace(parts[i])
		if t != "" {
			docs = append(docs, t)
		}
	}

	if len(docs) == 0 {
		step("from", fmt.Sprintf("no new documents: `%s`", filePath), false)
		return nil
	}

	step("from", fmt.Sprintf("file `%s` → %d new records", filePath, len(docs)), false)

	var matchArray []types.Item
	for _, doc := range docs {
		var rec types.FileRecord
		if err := yaml.Unmarshal([]byte(doc), &rec); err != nil {
			matchArray = append(matchArray, types.Item{
				ID:       generateID(),
				FilePath: item.FilePath,
				Text:     doc,
				Summary:  doc,
			})
			continue
		}

		if rec.Title == "" && rec.URL == "" && rec.Summary == "" && rec.Text == "" && rec.Timestamp == "" {
			matchArray = append(matchArray, types.Item{
				ID:       generateID(),
				FilePath: item.FilePath,
				Text:     doc,
				Summary:  doc,
			})
			continue
		}

		matchArray = append(matchArray, types.Item{
			ID:        generateID(),
			FilePath:  item.FilePath,
			URL:       rec.URL,
			Title:     rec.Title,
			Summary:   rec.Summary,
			Text:      rec.Text,
			Timestamp: rec.Timestamp,
		})
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(matchArray) == 0 {
		return nil
	}

	if len(minion.Keep) > 0 || len(minion.Ignore) > 0 {
		var dropWords []string
		for _, w := range minion.Ignore {
			dropWords = append(dropWords, strings.ToLower(w))
		}

		var keepWords []string
		for _, w := range minion.Keep {
			keepWords = append(keepWords, strings.ToLower(w))
		}

		var nextArray []types.Item
		for _, m := range matchArray {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			dropped := false
			content := strings.ToLower(fmt.Sprintf("%s %s %s", m.Text, m.Title, m.Summary))

			for _, word := range dropWords {
				if strings.Contains(content, word) {
					step("ignore", fmt.Sprintf("dropped `%s`", word), false)
					dropped = true
					break
				}
			}

			if !dropped && len(keepWords) > 0 {
				kept := false
				for _, word := range keepWords {
					if strings.Contains(content, word) {
						kept = true
						break
					}
				}
				if !kept {
					step("keep", "no match → dropped", false)
					dropped = true
				}
			}

			if !dropped {
				nextArray = append(nextArray, m)
			}
		}
		matchArray = nextArray
		if len(matchArray) == 0 {
			return nil
		}
	}

	if minion.Do != "" {
		var nextArray []types.Item
		for _, m := range matchArray {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			content := m.Text
			if content == "" {
				var parts []string
				if m.Title != "" {
					parts = append(parts, "Title: "+m.Title)
				}
				if m.Summary != "" {
					parts = append(parts, "Summary: "+m.Summary)
				}
				if m.URL != "" {
					parts = append(parts, "URL: "+m.URL)
				}
				if m.Timestamp != "" {
					parts = append(parts, "Timestamp: "+m.Timestamp)
				}
				if len(parts) > 0 {
					content = strings.Join(parts, "\n")
				}
			}

			if content == "" {
				continue
			}

			step("do", fmt.Sprintf("analyzing record"), false)
			runCtx.Stats.Analyzed++

			model := minion.Settings.Model
			if model == "" {
				model = os.Getenv("DEFAULT_MODEL")
			}
			if model == "" {
				runCtx.Stats.Errors++
				step("do", "no model configured: set DEFAULT_MODEL in .env or add model in settings", true)
				continue
			}

			currentDate := time.Now().Format("Monday, January 2, 2006 at 15:04 MST")

			systemPrompt := "You are a data record analyzer. Your job is to read the provided record and fulfill the user's task.\n\n"
			systemPrompt += fmt.Sprintf("CRITICAL TEMPORAL CONTEXT:\nToday's date and time is %s. Use this as your reference point for any time-based rules in the user's task.\n\n", currentDate)
			systemPrompt += "--- USER TASK START ---\n"
			systemPrompt += minion.Do + "\n"
			systemPrompt += "--- USER TASK END ---\n\n"
			systemPrompt += "MECHANICAL RULES:\n"
			systemPrompt += "- Extract ALL independent items from the record that fulfill the user's task.\n"
			systemPrompt += "- If the record provides a specific URL for the item, extract it into the 'url' field. Otherwise, leave it blank.\n"
			systemPrompt += "- If no items match the task, return an empty array for matches.\n"
			systemPrompt += "- You MUST output ONLY a valid JSON object matching this schema exactly:\n"
			systemPrompt += `{
  "matches": [
    {
      "title": "The name or title of the matched item.",
      "url": "The specific URL for the matched item (leave empty if none).",
      "summary": "A 1-sentence explanation of what the item is and why it matched. (Note: This is sent directly to the user, so write it for them to read using 'you' and 'your')."
    }
  ]
}`

			userMessage := ""
			if m.URL != "" {
				userMessage += fmt.Sprintf("--- SOURCE: %s ---\n\n", m.URL)
			} else {
				userMessage += "--- RECORD DATA ---\n\n"
			}
			userMessage += content

			evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
			res, cost, err := requestStructured[fileResult](evalCtx, model, systemPrompt, userMessage, llm.Chat)
			evalCancel()
			runCtx.Stats.TotalCost += cost

			if err != nil {
				runCtx.Stats.Errors++
				step("do", fmt.Sprintf("→ %v", err), true)
				continue
			}

			if len(res.Matches) == 0 {
				runCtx.Stats.Skipped++
				step("skip", "no matches", false)
			}

			runCtx.Stats.Results += len(res.Matches)

			nextArray = append(nextArray, buildFileResultItems(m, res.Matches)...)
		}
		matchArray = nextArray
		if len(matchArray) == 0 {
			return nil
		}
	}

	if len(minion.Tell) > 0 || runCtx.OnResult != nil {
		deliverTargets(ctx, minion, runCtx, matchArray, minion.Tell, true)
	}

	if !runCtx.Ephemeral {
		lastRaw := strings.TrimSpace(parts[len(parts)-1])
		lastHash := sha256.Sum256([]byte(lastRaw))
		newHash := fmt.Sprintf("%x", lastHash[:8])
		_ = runCtx.Store.UpdateFileHash(filePath, minion.Filename, newHash)
	}

	return nil
}

func buildFileResultItems(parentItem types.Item, matches []fileMatch) []types.Item {
	var items []types.Item
	for _, aiMatch := range matches {
		itemURL := aiMatch.URL
		if itemURL == "" {
			itemURL = parentItem.URL
		}
		itemURL = strings.TrimSuffix(itemURL, "/")
		cleanParentURL := strings.TrimSuffix(parentItem.URL, "/")
		items = append(items, types.Item{
			ID:        generateID(),
			URL:       itemURL,
			ParentURL: cleanParentURL,
			Title:     aiMatch.Title,
			Summary:   aiMatch.Summary,
		})
	}
	return items
}

func splitDocs(raw string) []string {
	var docs []string
	var buf []string
	for _, line := range strings.Split(raw, "\n") {
		if line == "---" || strings.HasPrefix(line, "--- ") {
			if len(buf) > 0 {
				docs = append(docs, strings.Join(buf, "\n"))
				buf = nil
			}
			continue
		}
		buf = append(buf, line)
	}
	if len(buf) > 0 {
		docs = append(docs, strings.Join(buf, "\n"))
	}
	return docs
}
