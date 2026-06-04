package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"minion/internal/config"
	"minion/internal/llm"
	"minion/internal/types"
)

type commandMatch struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

type commandResult struct {
	Matches []commandMatch `json:"matches"`
}

func processCommandItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	command := item.Command
	if command == "" {
		return nil
	}

	isDiscarded, err := runCtx.Store.IsDiscarded(command, minion.Filename)
	if err == nil && isDiscarded {
		step("discarded", fmt.Sprintf("already discarded: `%s`", command), false)
		return nil
	}

	timeoutSec := minion.Settings.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	expandedCmd := strictExpandEnv(command)

	step("fetch", fmt.Sprintf("running `%s`", command), false)

	execCtx, execCancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	out, runErr := exec.CommandContext(execCtx, "sh", "-c", expandedCmd).CombinedOutput()
	execCancel()

	exitCode := 0
	if runErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			runCtx.Stats.Errors++
			step("fetch", fmt.Sprintf("command timed out after %ds: `%s`", timeoutSec, command), true)
			return nil
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			runCtx.Stats.Errors++
			step("fetch", fmt.Sprintf("command failed: `%s`: %v", command, runErr), true)
			return nil
		}
	}

	outputText := string(out)
	if strings.TrimSpace(outputText) == "" {
		step("fetch", "command produced no output", false)
	}

	h := sha256.Sum256([]byte(strings.ToLower(outputText)))
	hash := fmt.Sprintf("%x", h[:8])

	savedHash, _ := runCtx.Store.GetPageHash(command, minion.Filename)
	if savedHash == hash {
		runCtx.Stats.Unchanged++
		step("unchanged", "skipped", false)
		return nil
	}

	_ = runCtx.Store.UpdatePageHash(command, minion.Filename, hash)

	runCtx.Stats.Fetched++

	m := *item
	m.Text = outputText
	m.TempHash = hash
	matchArray := []types.Item{m}

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
					step("keep", "no match -> dropped", false)
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
				content = fmt.Sprintf("Command: %s\nExit code: %d\n(no output)", command, exitCode)
			}

			step("do", fmt.Sprintf("analyzing output of `%s`", command), false)
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

			systemP := "You are a command output analyzer. Your job is to read the provided shell command output and fulfill the user's task.\n\n"
			systemP += fmt.Sprintf("CRITICAL TEMPORAL CONTEXT:\nToday's date and time is %s. Use this as your reference point for any time-based rules in the user's task.\n\n", currentDate)
			systemP += fmt.Sprintf("CONTEXT:\n  Exit code: %d\n\n", exitCode)
			systemP += "--- USER TASK START ---\n"
			systemP += minion.Do + "\n"
			systemP += "--- USER TASK END ---\n\n"
			systemP += "MECHANICAL RULES:\n"
			systemP += "- Extract ALL independent items from the command output that fulfill the user's task.\n"
			systemP += "- If the output mentions a specific [Link: URL] or URL for the item, extract it into the 'url' field. Otherwise, leave it blank.\n"
			systemP += "- Return multiple matches when the output is structured data (JSON arrays, tables, lists of records). For everything else, return one match with the full analysis.\n"
			systemP += "- If no items match the task, return an empty array for matches.\n"
			systemP += "- You MUST output ONLY a valid JSON object matching this schema exactly:\n"
			systemP += `{
  "matches": [
    {
      "title": "This is the name or title of the matched item. Look at the USER TASK to see anything you need to modify or incorporate to this field to fulfill the instructions. Default to the exact name or title of the item, but prioritize the USER TASK if there is any conflict.",
      "url": "This is the specific URL for the matched item. Look at the USER TASK to see anything you need to modify or incorporate to this field to fulfill the instructions. Default to the exact URL found in the text (leave empty if none), but prioritize the USER TASK if there is any conflict.",
      "summary": "This is the summary of the matched item. Look at the USER TASK to see anything you need to add or incorporate to this field to fulfill the instructions. Default to a 1-sentence explanation of what the item is and why it matched, but prioritize the USER TASK if there is any conflict. (Note: This is sent directly to the user, so write it for them to read using 'you' and 'your')."
    }
  ]
}`

			userMessage := fmt.Sprintf("--- COMMAND: %s ---\n\n", command)
			userMessage += content

			evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
			raw, cost, err := llm.Chat(evalCtx, model, systemP, userMessage, true)
			evalCancel()

			if err != nil {
				runCtx.Stats.Errors++
				step("do", fmt.Sprintf("`%s` -> %v", command, err), true)
				continue
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

			var res commandResult
			if err := json.Unmarshal([]byte(raw), &res); err != nil {
				runCtx.Stats.Errors++
				step("do", fmt.Sprintf("failed to parse llm output: %v", err), true)
				continue
			}

			if len(res.Matches) == 0 {
				runCtx.Stats.Skipped++
				step("skip", fmt.Sprintf("no matches on `%s`", command), false)
			}

			runCtx.Stats.Results += len(res.Matches)

			nextArray = buildCommandResultItems(res.Matches)
		}
		matchArray = nextArray
		if len(matchArray) == 0 {
			return nil
		}
	}

	if len(minion.Tell) > 0 {
		deliverTargets(ctx, minion, runCtx, matchArray, minion.Tell, true)
	}

	return nil
}

func buildCommandResultItems(matches []commandMatch) []types.Item {
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
