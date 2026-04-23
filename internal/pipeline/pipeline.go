package pipeline

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"minion/internal/config"
	"minion/internal/llm"
	"minion/internal/scraper"
	"minion/internal/store"
	"minion/internal/webhook"
)

// RunContext holds the necessary components for a pipeline run.
type RunContext struct {
	Store *store.Store
	LLM   *llm.Evaluator
	// Optional callback for UI updates
	OnStep func(step, details string, isError bool)
}

// RunMinion executes a single minion's pipeline.
func RunMinion(ctx context.Context, minion *config.MinionConfig, runCtx *RunContext) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	step("GATHER", fmt.Sprintf("Starting %s", minion.Name), false)

	// 1. Gather Sources
	var allURLs []string
	
	for _, src := range minion.Sources {
		if src.FollowLinks != "" {
			step("SEARCH", fmt.Sprintf("Extracting '%s' links from: %s", src.FollowLinks, src.URL), false)
			links, err := scraper.ExtractLinks(src.URL, src.FollowLinks)
			if err != nil {
				step("SEARCH ERROR", err.Error(), true)
			} else {
				step("SEARCH", fmt.Sprintf("Found %d matching links", len(links)), false)
				allURLs = append(allURLs, links...)
			}
		} else {
			allURLs = append(allURLs, src.URL)
		}
	}

	if minion.WebSearch != nil {
		// Initialize random seed for human-like jitter
		rand.Seed(time.Now().UnixNano())

		for i, q := range minion.WebSearch.Queries {
			// Add a random delay (1 to 3 seconds) between searches to avoid rate limiting
			if i > 0 {
				jitter := time.Duration(rand.Intn(3)+1) * time.Second
				step("DELAY", fmt.Sprintf("Waiting %v (search jitter)...", jitter), false)
				time.Sleep(jitter)
			}

			step("SEARCH", fmt.Sprintf("Query: %s", q), false)
			urls, err := scraper.SearchDuckDuckGo(q, minion.WebSearch.MaxResultsPerQuery)
			if err != nil {
				step("SEARCH ERROR", err.Error(), true)
			} else {
				allURLs = append(allURLs, urls...)
			}
		}
	}

	// 2. Process URLs
	for i, targetURL := range allURLs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Add a tiny random delay (1-2 seconds) between actual webpage scrapes 
		// to avoid triggering DDoS protections on the target servers, 
		// especially if follow_links generated a massive list on the same domain.
		if i > 0 {
			jitter := time.Duration(rand.Intn(2)+1) * time.Second
			step("DELAY", fmt.Sprintf("Waiting %v (scrape jitter)...", jitter), false)
			time.Sleep(jitter)
		}

		// Scrape
		step("SCRAPE", targetURL, false)
		text, err := scraper.FetchAndSanitize(targetURL)
		if err != nil {
			step("SCRAPE ERROR", fmt.Sprintf("Failed %s: %v", targetURL, err), true)
			continue
		}

		// Dumb Filter
		lowerText := strings.ToLower(text)
		filtered := false
		for _, skipWord := range minion.SkipIfContains {
			if strings.Contains(lowerText, strings.ToLower(skipWord)) {
				step("FILTER", fmt.Sprintf("Dropped %s due to word: %s", targetURL, skipWord), false)
				filtered = true
				break
			}
		}

		if filtered {
			continue
		}

		// LLM Eval
		step("EVAL", fmt.Sprintf("Evaluating %s", targetURL), false)
		res, err := runCtx.LLM.EvaluateText(ctx, text, minion.Task)
		if err != nil {
			step("EVAL ERROR", fmt.Sprintf("LLM failed for %s: %v", targetURL, err), true)
			continue
		}

		if len(res.Matches) > 0 {
			matchCount := 0
			for _, match := range res.Matches {
				// Deduplicate by Event, not by URL
				hashID := store.GenerateHash(minion.Name, targetURL, match.Title)
				notified, err := runCtx.Store.HasNotified(hashID)
				if err != nil {
					step("STORE ERROR", fmt.Sprintf("DB check failed: %v", err), true)
					continue
				}
				
				if notified {
					// We've already alerted the user about this specific event
					continue
				}

				matchCount++
				step("ITEM", fmt.Sprintf("%s: %s", match.Title, match.Summary), false)
				
				if minion.Webhook != nil && minion.Webhook.URL != "" {
					title := fmt.Sprintf("%s: %s", minion.Name, match.Title)
					if len(title) > 64 {
						title = title[:61] + "..." // some services restrict title length
					}
					
					clickURL := targetURL
					if match.URL != "" {
						clickURL = match.URL
					}
					
					err := webhook.Send(minion.Webhook, title, match.Summary, clickURL)
					if err != nil {
						step("NOTIFY ERROR", err.Error(), true)
					} else {
						step("NOTIFY", fmt.Sprintf("Sent notification for '%s'", match.Title), false)
						// Mark as notified in the database so we never send it again
						_ = runCtx.Store.MarkNotified(hashID, minion.Name)
					}
				} else {
					// Even if there's no notify URL, we mark it to prevent terminal spam on next run
					_ = runCtx.Store.MarkNotified(hashID, minion.Name)
				}
			}
			
			if matchCount > 0 {
				step("MATCH", fmt.Sprintf("Found %d NEW matches!", matchCount), false)
			} else {
				step("MATCH", "Found matches, but all were already notified.", false)
			}

		} else {
			step("NO MATCH", "No matches found on this page.", false)
		}
	}

	step("DONE", fmt.Sprintf("Finished %s", minion.Name), false)
	return nil
}