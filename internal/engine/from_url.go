package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"minion/internal/config"
	"minion/internal/llm"
	"minion/internal/scraper"
	"minion/internal/types"
)

type urlMatch struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

type urlResult struct {
	CacheAction string     `json:"cache_action"`
	Matches     []urlMatch `json:"matches"`
}

func processURLItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	var matchArray []types.Item
	matchArray = append(matchArray, *item)

	timeoutSec := minion.Settings.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	delaySec := minion.Settings.Delay
	if delaySec <= 0 {
		delaySec = 2
	}

	var scrapedArray []types.Item
	for _, m := range matchArray {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if m.URL == "" {
			scrapedArray = append(scrapedArray, m)
			continue
		}

		isDiscarded, err := runCtx.Store.IsDiscarded(m.URL, minion.Filename)
		if err == nil && isDiscarded {
			step("discarded", fmt.Sprintf("already discarded: `%s`", m.URL), false)
			continue
		}

		if m.Text != "" && m.TempHash != "" {
			step("fetch", fmt.Sprintf("passed through: `%s`", m.URL), false)

			savedHash, _ := runCtx.Store.GetPageHash(m.URL, minion.Filename)
			if savedHash == m.TempHash {
				runCtx.Stats.Unchanged++
				step("unchanged", "skipped", false)
				continue
			}

			_ = runCtx.Store.UpdatePageHash(m.URL, minion.Filename, m.TempHash)
			scrapedArray = append(scrapedArray, m)
			continue
		}

		if delaySec > 0 {
			jitter := rand.Intn(delaySec) + 1
			time.Sleep(time.Duration(jitter) * time.Second)
		}

		step("fetch", fmt.Sprintf("retrieving `%s`", m.URL), false)
		var text, hash string
		var fetchErr error
		if m.Render {
			browser, bErr := runCtx.GetBrowser(timeoutSec)
			if bErr != nil {
				runCtx.Stats.Errors++
				step("fetch", bErr.Error(), true)
				continue
			}
			text, hash, fetchErr = scraper.FetchRenderedAndSanitize(ctx, browser, m.URL, timeoutSec)
		} else {
			text, hash, fetchErr = scraper.FetchAndSanitize(ctx, m.URL, timeoutSec)
		}
		if fetchErr != nil {
			runCtx.Stats.Errors++
			step("fetch", fetchErr.Error(), true)
			continue
		}

		runCtx.Stats.Fetched++

		savedHash, _ := runCtx.Store.GetPageHash(m.URL, minion.Filename)
		if savedHash == hash {
			runCtx.Stats.Unchanged++
			step("unchanged", "skipped", false)
			continue
		}

		_ = runCtx.Store.UpdatePageHash(m.URL, minion.Filename, hash)

		m.Text = text
		m.TempHash = hash
		scrapedArray = append(scrapedArray, m)
	}
	matchArray = scrapedArray
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
			content := strings.ToLower(fmt.Sprintf("%s %s %s %s", m.URL, m.Text, m.Title, m.Summary))

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
				if m.URL != "" {
					content = "URL: " + m.URL
				} else {
					content = minion.Do
				}
			}

			step("do", fmt.Sprintf("analyzing `%s` for matches", m.URL), false)
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

			systemPrompt := "You are a web page analyzer. Your job is to read the provided web page content and fulfill the user's task.\n\n"
			systemPrompt += fmt.Sprintf("CRITICAL TEMPORAL CONTEXT:\nToday's date and time is %s. Use this as your reference point for any time-based rules in the user's task.\n\n", currentDate)
			systemPrompt += "--- USER TASK START ---\n"
			systemPrompt += minion.Do + "\n"
			systemPrompt += "--- USER TASK END ---\n\n"
			systemPrompt += "MECHANICAL RULES:\n"
			systemPrompt += "- Extract ALL independent items from the text that fulfill the user's task.\n"
			systemPrompt += "- If the text provides a specific [Link: URL] for the item, extract it into the 'url' field. Otherwise, leave it blank.\n"
			systemPrompt += "- If no items match the task, return an empty array for matches.\n"
			systemPrompt += "- CACHING & ROUTING (CRITICAL): The user may use words like \"discard\", \"skip\", or \"ignore\" in their task to filter items. You must be aware of this and NOT confuse their item-level filtering with your page-level routing. Evaluate the page independently based on these consequences:\n"
			systemPrompt += "  * Return 'discard' if the page is permanently off-topic, junk, or old. CONSEQUENCE: You will permanently blacklist this URL and we will NEVER ask you to read it again.\n"
			systemPrompt += "  * Return 'skip' for lists that update frequently, or for relevant pages that simply fail a dynamic rule (like how \"next 5 days\" or prices change every day). CONSEQUENCE: You will ignore the page for now, but we will ask you to check it again on the next run because either the page content might change, OR the current date/time will change making the rules match later.\n"
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

			userMessage := ""
			if m.URL != "" {
				userMessage += fmt.Sprintf("--- SOURCE: %s ---\n\n", m.URL)
			}
			userMessage += content

			evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
			raw, cost, err := llm.Chat(evalCtx, model, systemPrompt, userMessage, true)
			evalCancel()

			if err != nil {
				runCtx.Stats.Errors++
				step("do", fmt.Sprintf("`%s` → %v", m.URL, err), true)
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

			var res urlResult
			if err := json.Unmarshal([]byte(raw), &res); err != nil {
				runCtx.Stats.Errors++
				step("do", fmt.Sprintf("failed to parse llm output: %v", err), true)
				continue
			}

			if res.CacheAction == "discard" {
				if m.Protected {
					runCtx.Stats.Skipped++
					step("discard", fmt.Sprintf("protected: `%s`", m.URL), false)
				} else {
					runCtx.Stats.Discarded++
					step("discard", fmt.Sprintf("irrelevant: `%s`", m.URL), false)
					_ = runCtx.Store.MarkDiscarded(m.URL, minion.Filename)
				}
			} else if len(res.Matches) == 0 {
				runCtx.Stats.Skipped++
				step("skip", fmt.Sprintf("no matches on `%s`", m.URL), false)
			}

			runCtx.Stats.Results += len(res.Matches)

			for _, aiMatch := range res.Matches {
				itemURL := aiMatch.URL
				if itemURL == "" {
					itemURL = m.URL
				}

				itemURL = strings.TrimSuffix(itemURL, "/")
				cleanParentURL := strings.TrimSuffix(m.URL, "/")

				inheritedText := ""
				inheritedHash := ""
				if itemURL == cleanParentURL {
					inheritedText = m.Text
					inheritedHash = m.TempHash
				}

				nextArray = append(nextArray, types.Item{
					ID:        generateID(),
					URL:       itemURL,
					ParentURL: cleanParentURL,
					Title:     aiMatch.Title,
					Summary:   aiMatch.Summary,
					Text:      inheritedText,
					TempHash:  inheritedHash,
				})
			}
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
