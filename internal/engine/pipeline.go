package engine

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
	
	"gopkg.in/yaml.v3"

	"minion/internal/config"
	"minion/internal/llm"
	"minion/internal/scraper"
	"minion/internal/store"
	"minion/internal/types"
	"minion/internal/delivery"
)

type Stats struct {
	StartTime      time.Time
	EndTime        time.Time
	PagesScraped   int
	PagesCached    int
	LLMEvals       int
	ItemsFound     int
	ItemsDelivered int
	Errors         int
}

func (s *Stats) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

func (s *Stats) GenerateReport() string {
	return fmt.Sprintf(
		"Start: %s\n"+
			"End:   %s\n"+
			"Time:  %s\n\n"+
			"Pages Fetched:   %d\n"+
			"Pages Cached:    %d\n"+
			"LLM Evaluations: %d\n"+
			"Items Found:     %d\n"+
			"Items Delivered: %d\n"+
			"Errors:          %d",
		s.StartTime.Format("2006-01-02 15:04:05"),
		s.EndTime.Format("15:04:05"),
		s.Duration().Round(time.Millisecond*100),
		s.PagesScraped, s.PagesCached, s.LLMEvals,
		s.ItemsFound, s.ItemsDelivered, s.Errors,
	)
}

type RunContext struct {
	Store  *store.Store
	LLM    *llm.Evaluator
	Stats  *Stats
	OnStep func(step, details string, isError bool)
}

func RunMission(ctx context.Context, minion *config.MinionConfig, runCtx *RunContext) error {
	_ = runCtx.Store.MarkJobActive(minion.Filename)
	defer runCtx.Store.MarkJobDone(minion.Filename)

	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	step("MISSION", fmt.Sprintf("Starting %s", minion.Name), false)

	if runCtx.Stats == nil {
		runCtx.Stats = &Stats{StartTime: time.Now()}
	}

	var startingURLs []string

	// 1. GENERATORS (Gather all URLs first)
	rand.Seed(time.Now().UnixNano())

	for _, action := range minion.Mission {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if val, ok := action["search"]; ok {
			limit := 3
			if l, ok := action["limit"].(int); ok {
				limit = l
			}

			var queries []string
			if queryStr, ok := val.(string); ok {
				queries = append(queries, queryStr)
			} else if queryList, ok := val.([]interface{}); ok {
				for _, q := range queryList {
					queries = append(queries, fmt.Sprintf("%v", q))
				}
			}

			for i, q := range queries {
				if i > 0 {
					time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
				}
				step("SEARCH", fmt.Sprintf("Query: %s", q), false)
				urls, err := scraper.SearchDuckDuckGo(q, limit)
				if err != nil {
					runCtx.Stats.Errors++
					step("SEARCH ERROR", err.Error(), true)
					continue
				}
				startingURLs = append(startingURLs, urls...)
			}
			continue
		}

		if val, ok := action["browse"]; ok {
			if browseList, ok := val.([]interface{}); ok {
				for _, c := range browseList {
					if cmap, ok := c.(map[string]interface{}); ok {
						u, _ := cmap["url"].(string)
						if u == "" {
							continue
						}
						
						matchPattern := ""
						if m, ok := cmap["match"].(string); ok {
							matchPattern = m
						}

						if matchPattern == "" {
							startingURLs = append(startingURLs, u)
							step("BROWSE", fmt.Sprintf("Added %s", u), false)
						} else {
							step("BROWSE", fmt.Sprintf("Scanning %s for regex '%s'", u, matchPattern), false)
							links, err := scraper.ExtractLinks(u, matchPattern)
							if err != nil {
								runCtx.Stats.Errors++
								step("BROWSE ERROR", err.Error(), true)
								continue
							}
							startingURLs = append(startingURLs, links...)
						}
					} else if u, ok := c.(string); ok {
						startingURLs = append(startingURLs, u)
						step("BROWSE", fmt.Sprintf("Added %s", u), false)
					}
				}
			}
			continue
		}
	}

	// 2. THE LINEAR STREAM (Process one by one)
	for i, u := range startingURLs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		item := &types.Item{URL: u}
		err := ProcessItem(ctx, minion, item, runCtx, i > 0, "cron")
		if err != nil {
			runCtx.Stats.Errors++
			step("ERROR", fmt.Sprintf("Failed processing %s: %v", u, err), true)
		}
	}

	runCtx.Stats.EndTime = time.Now()

	// Check for a report action to deliver the stats
	for _, action := range minion.Mission {
		if val, ok := action["report"]; ok {
			var targets []map[string]interface{}
			if tList, ok := val.([]interface{}); ok {
				for _, t := range tList {
					if tMap, ok := t.(map[string]interface{}); ok {
						targets = append(targets, tMap)
					}
				}
			}

			reportText := runCtx.Stats.GenerateReport()
			reportItem := types.Item{
				Title:   fmt.Sprintf("Mission Report: %s", minion.Name),
				Summary: reportText,
			}

			step("REPORT", "Delivering mission report", false)
			deliverTargets(ctx, minion, runCtx, []types.Item{reportItem}, targets, false)
			break
		}
	}

	step("DONE", fmt.Sprintf("Finished %s", minion.Name), false)
	return nil
}

// ProcessItem pushes a single item through the transformer and delivery steps.
func ProcessItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext, requireJitter bool, parentName string) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	var matchArray []types.Item
	matchArray = append(matchArray, *item)

	for _, action := range minion.Mission {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Security Checkpoint (Receive)
		if val, ok := action["receive"]; ok {
			expectedParent := ""
			if valStr, ok := val.(string); ok {
				expectedParent = valStr
			} else if receiveList, ok := val.([]interface{}); ok {
				// Handle array format e.g., - receive: [{minion: "Master"}]
				for _, r := range receiveList {
					if rMap, ok := r.(map[string]interface{}); ok {
						if mName, ok := rMap["minion"].(string); ok {
							expectedParent = mName
						}
					}
				}
			}

			if parentName != expectedParent {
				step("REJECTED", fmt.Sprintf("Expected data from '%s', but received from '%s'", expectedParent, parentName), true)
				return nil // Stop processing this item, unauthorized
			}
			
			step("RECEIVE", fmt.Sprintf("Verified connection from '%s'", parentName), false)
			continue
		}

		// Skip generators
		if _, ok := action["search"]; ok { continue }
		if _, ok := action["browse"]; ok { continue }
		if _, ok := action["schedule"]; ok { continue }

		if val, ok := action["filter"]; ok {
			var dropWords []string
			if dw, ok := action["drop_if_contains"].([]interface{}); ok {
				for _, w := range dw {
					dropWords = append(dropWords, strings.ToLower(fmt.Sprintf("%v", w)))
				}
			} else if dw, ok := val.([]interface{}); ok {
				// Allow simple array syntax: filter: ["paywall"]
				for _, w := range dw {
					dropWords = append(dropWords, strings.ToLower(fmt.Sprintf("%v", w)))
				}
			}

			var nextArray []types.Item
			for _, m := range matchArray {
				dropped := false
				content := strings.ToLower(fmt.Sprintf("%s %s %s %s", m.URL, m.Text, m.Title, m.Summary))
				for _, word := range dropWords {
					if strings.Contains(content, word) {
						step("FILTER", fmt.Sprintf("Dropped %s due to '%s'", m.URL, word), false)
						dropped = true
						break
					}
				}
				if !dropped {
					nextArray = append(nextArray, m)
				}
			}
			matchArray = nextArray
			if len(matchArray) == 0 { return nil } // Fast exit
			continue
		}

		if _, ok := action["scrape"]; ok {
			var nextArray []types.Item
			for _, m := range matchArray {
				if m.URL == "" { continue }

				isDropped, err := runCtx.Store.IsDropped(m.URL, minion.Filename)
				if err == nil && isDropped {
					step("CACHED", fmt.Sprintf("Skipping dropped URL: %s", m.URL), false)
					continue
				}

				if requireJitter {
					time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second)
					requireJitter = false // only jitter once per URL stream
				}

				step("SCRAPE", m.URL, false)
				text, hash, err := scraper.FetchAndSanitize(m.URL)
				if err != nil {
					runCtx.Stats.Errors++
					step("SCRAPE ERROR", err.Error(), true)
					continue
				}

				runCtx.Stats.PagesScraped++

				savedHash, _ := runCtx.Store.GetPageHash(m.URL, minion.Filename)
				if savedHash == hash {
					runCtx.Stats.PagesCached++
					step("CACHED", "Page content unchanged, skipping LLM", false)
					continue
				}

				m.Text = text
				m.TempHash = hash
				nextArray = append(nextArray, m)
			}
			matchArray = nextArray
			if len(matchArray) == 0 { return nil }
			continue
		}

		if _, ok := action["study"]; ok {
			task, _ := action["task"].(string)
			format, _ := action["format"].(string)
			if format == "" {
				format = "json_list"
			}

			var nextArray []types.Item
			for _, m := range matchArray {
				content := m.Text
				if content == "" {
					content = "URL: " + m.URL
				}

				step("STUDY", fmt.Sprintf("Reading %s", m.URL), false)
				runCtx.Stats.LLMEvals++
				
				evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
				res, err := runCtx.LLM.EvaluateText(evalCtx, content, task, format)
				evalCancel()

				if err != nil {
					runCtx.Stats.Errors++
					step("STUDY ERROR", fmt.Sprintf("LLM failed for %s: %v", m.URL, err), true)
					continue
				}

				if res.CacheAction == "permanent_drop" {
					step("CACHE", fmt.Sprintf("AI marked %s for permanent drop", m.URL), false)
					_ = runCtx.Store.MarkDropped(m.URL, minion.Filename)
				} else if res.CacheAction == "re_evaluate_later" {
					step("KEEP", fmt.Sprintf("AI marked %s for re-evaluation later", m.URL), false)
				}

				runCtx.Stats.ItemsFound += len(res.Matches)

				for _, aiMatch := range res.Matches {
					nextArray = append(nextArray, types.Item{
						URL:      aiMatch.URL,
						Title:    aiMatch.Title,
						Summary:  aiMatch.Summary,
						Text:     m.Text, // Preserve text for downstream filters
						TempHash: m.TempHash,
					})
				}
			}
			matchArray = nextArray
			if len(matchArray) == 0 { return nil }
			continue
		}

		if val, ok := action["deliver"]; ok {
			var targets []map[string]interface{}
			
			if tList, ok := val.([]interface{}); ok {
				for _, t := range tList {
					if tMap, ok := t.(map[string]interface{}); ok {
						targets = append(targets, tMap)
					}
				}
			}

			deliverTargets(ctx, minion, runCtx, matchArray, targets, true)
			continue
		}
	}

	return nil
}

func deliverTargets(ctx context.Context, minion *config.MinionConfig, runCtx *RunContext, matchArray []types.Item, targets []map[string]interface{}, saveHash bool) {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	pagesDelivered := make(map[string]string)

	for _, m := range matchArray {
		if saveHash {
			step("ITEM", fmt.Sprintf("%s: %s", m.Title, m.Summary), false)
		}

		for _, t := range targets {
			if ntfyURL, ok := t["ntfy"]; ok {
				urlStr := fmt.Sprintf("%v", ntfyURL)
				var auth *delivery.BasicAuth
				if baData, ok := t["basic_auth"]; ok {
					b, _ := yaml.Marshal(baData)
					yaml.Unmarshal(b, &auth)
				}
				
				err := delivery.SendNtfy(urlStr, auth, minion.Name, &m)
				if err != nil {
					runCtx.Stats.Errors++
					step("DELIVERY ERROR", fmt.Sprintf("ntfy: %v", err), true)
				} else {
					if saveHash { runCtx.Stats.ItemsDelivered++ }
					step("DELIVERY", fmt.Sprintf("Sent ntfy alert for '%s'", m.Title), false)
				}
			}

			if discordURL, ok := t["discord"]; ok {
				urlStr := fmt.Sprintf("%v", discordURL)
				err := delivery.SendDiscord(urlStr, minion.Name, &m)
				if err != nil {
					runCtx.Stats.Errors++
					step("DELIVERY ERROR", fmt.Sprintf("discord: %v", err), true)
				} else {
					if saveHash { runCtx.Stats.ItemsDelivered++ }
					step("DELIVERY", fmt.Sprintf("Sent discord alert for '%s'", m.Title), false)
				}
			}

			if reqURL, ok := t["http_request"]; ok {
				urlStr := fmt.Sprintf("%v", reqURL)
				wbConfig := &delivery.HTTPRequestConfig{URL: urlStr}
				
				if method, ok := t["method"].(string); ok {
					wbConfig.Method = method
				}
				if tmpl, ok := t["payload_template"].(string); ok {
					wbConfig.PayloadTemplate = tmpl
				}
				if headers, ok := t["headers"].(map[string]interface{}); ok {
					wbConfig.Headers = make(map[string]string)
					for k, v := range headers {
						wbConfig.Headers[k] = fmt.Sprintf("%v", v)
					}
				}
				if baData, ok := t["basic_auth"]; ok {
					b, _ := yaml.Marshal(baData)
					var ba delivery.BasicAuth
					yaml.Unmarshal(b, &ba)
					wbConfig.BasicAuth = &ba
				}

				err := delivery.SendHTTPRequest(wbConfig, &m)
				if err != nil {
					runCtx.Stats.Errors++
					step("DELIVERY ERROR", fmt.Sprintf("http_request: %v", err), true)
				} else {
					if saveHash { runCtx.Stats.ItemsDelivered++ }
					step("DELIVERY", fmt.Sprintf("Sent custom HTTP request for '%s'", m.Title), false)
				}
			}

			if minionNameRaw, ok := t["minion"]; ok {
				targetMinionName := fmt.Sprintf("%v", minionNameRaw)
				step("CHAIN", fmt.Sprintf("Delivering item to minion '%s'", targetMinionName), false)
				
				targetMinion, err := config.LoadMinion(targetMinionName)
				if err != nil {
					runCtx.Stats.Errors++
					step("CHAIN ERROR", fmt.Sprintf("Failed to load target minion '%s': %v", targetMinionName, err), true)
				} else {
					err = ProcessItem(ctx, targetMinion, &m, runCtx, false, minion.Filename)
					if err != nil {
						runCtx.Stats.Errors++
						step("CHAIN ERROR", fmt.Sprintf("Target minion '%s' failed: %v", targetMinionName, err), true)
					} else {
						if saveHash { runCtx.Stats.ItemsDelivered++ }
					}
				}
			}
		}
		
		if m.URL != "" && m.TempHash != "" {
			pagesDelivered[m.URL] = m.TempHash
		}
	}

	if saveHash {
		for url, hash := range pagesDelivered {
			_ = runCtx.Store.UpdatePageHash(url, minion.Filename, hash)
		}
	}
}
