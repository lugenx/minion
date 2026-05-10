package engine

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"
	
	"gopkg.in/yaml.v3"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

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
	SearchLinks    int
	BrowseLinks    int
	PagesScraped   int
	PagesCached    int
	PagesStudied   int
	PagesDiscarded int
	PagesSkipped   int
	ItemsFound     int
	ItemsDelivered int
	Errors         int
	TotalCost      float64
}

func (s *Stats) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

func (s *Stats) GenerateReport(minionName string) string {
	costStr := fmt.Sprintf("$%.4f", s.TotalCost)
	if s.TotalCost == 0 {
		costStr = "$0.0000"
	}

	return fmt.Sprintf(
		"Mission Report: %s\n\n"+
			"Start:  %s\n"+
			"End:    %s\n"+
			"Time:   %s\n"+
			"Errors: %d\n\n"+
			"- Search Results: %d links\n"+
			"- Browse Results: %d links\n"+
			"- Scraped:        %d pages\n"+
			"    - Cached:     %d pages\n"+
			"- Studied:        %d pages\n"+
			"    - Discarded:  %d pages\n"+
			"    - Skipped:    %d pages\n"+
			"    - Found:      %d items\n"+
			"- Delivered:      %d items\n\n"+
			"Cost:   %s",
		minionName,
		s.StartTime.Format("2006-01-02 15:04:05"),
		s.EndTime.Format("15:04:05"),
		s.Duration().Round(time.Millisecond*100),
		s.Errors,
		s.SearchLinks, s.BrowseLinks,
		s.PagesScraped, s.PagesCached,
		s.PagesStudied, s.PagesDiscarded, s.PagesSkipped,
		s.ItemsFound, s.ItemsDelivered,
		costStr,
	)
}

type RunContext struct {
	Store      *store.Store
	LLM        *llm.Evaluator
	Stats      *Stats
	OnStep     func(step, details string, isError bool)
	SmartSplit map[string]string
	Browser    *rod.Browser
	Launcher   *launcher.Launcher
}

func (r *RunContext) GetBrowser(timeoutSec int) (*rod.Browser, error) {
	if r.Browser == nil {
		if timeoutSec <= 0 {
			timeoutSec = 15
		}
		l := launcher.New()
		u, err := l.Launch()
		if err != nil {
			return nil, fmt.Errorf("failed to launch chromium: %w", err)
		}
		
		browser := rod.New().ControlURL(u)
		if err := browser.Connect(); err != nil {
			l.Cleanup()
			return nil, fmt.Errorf("failed to start headless browser: %w", err)
		}
		
		r.Launcher = l
		r.Browser = browser
	}
	return r.Browser, nil
}

func RunMission(ctx context.Context, minion *config.MinionConfig, runCtx *RunContext) error {
	_ = runCtx.Store.MarkJobActive(minion.Filename)
	defer runCtx.Store.MarkJobDone(minion.Filename)
	defer func() {
		if runCtx.Browser != nil {
			_ = runCtx.Browser.Close()
			runCtx.Browser = nil
		}
		if runCtx.Launcher != nil {
			runCtx.Launcher.Cleanup()
			runCtx.Launcher = nil
		}
	}()

	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	step("MISSION", fmt.Sprintf("Starting %s", minion.Name), false)

	if runCtx.Stats == nil {
		runCtx.Stats = &Stats{StartTime: time.Now()}
	}

	// Extract global timeout from scrape block (defaults to 30s for headless operations)
	globalTimeout := 30
	for _, action := range minion.Mission {
		if scrapeVal, ok := action["scrape"]; ok {
			if scrapeMap, ok := scrapeVal.(map[string]interface{}); ok {
				if t, ok := scrapeMap["timeout"].(int); ok {
					if t > 0 {
						globalTimeout = t
					}
				}
			}
			break
		}
	}

	var startingURLs []string
	protectedURLs := make(map[string]bool)
	renderURLs := make(map[string]bool)

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
				urls, err := scraper.SearchDuckDuckGo(q, limit, 15)
				if err != nil {
					runCtx.Stats.Errors++
					step("SEARCH ERROR", err.Error(), true)
					continue
				}
				runCtx.Stats.SearchLinks += len(urls)
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
						
						renderFlag := false
						if r, ok := cmap["render"].(bool); ok {
							renderFlag = r
						}

						matchPattern := ""
						if m, ok := cmap["match"].(string); ok {
							matchPattern = m
						}

						if matchPattern == "" {
							startingURLs = append(startingURLs, u)
							protectedURLs[u] = true
							if renderFlag {
								renderURLs[u] = true
							}
							runCtx.Stats.BrowseLinks++
							step("BROWSE", fmt.Sprintf("Added %s", u), false)
						} else {
							step("BROWSE", fmt.Sprintf("Scanning %s for regex '%s'", u, matchPattern), false)
							var links []string
							var err error
							if renderFlag {
								browser, bErr := runCtx.GetBrowser(globalTimeout)
								if bErr != nil {
									runCtx.Stats.Errors++
									step("BROWSE ERROR", bErr.Error(), true)
									continue
								}
								links, err = scraper.ExtractLinksRendered(browser, u, matchPattern, globalTimeout)
							} else {
								links, err = scraper.ExtractLinks(u, matchPattern, globalTimeout)
							}
							if err != nil {
								runCtx.Stats.Errors++
								step("BROWSE ERROR", err.Error(), true)
								continue
							}
							runCtx.Stats.BrowseLinks += len(links)
							startingURLs = append(startingURLs, links...)
							if renderFlag {
								for _, link := range links {
									renderURLs[link] = true
								}
							}
						}
					} else if u, ok := c.(string); ok {
						startingURLs = append(startingURLs, u)
						protectedURLs[u] = true
						runCtx.Stats.BrowseLinks++
						step("BROWSE", fmt.Sprintf("Added %s", u), false)
					}
				}
			}
			continue
		}
	}

	// 2. THE LINEAR STREAM (Process one by one)
	// Deduplicate startingURLs to prevent processing the same URL multiple times in one run
	var uniqueURLs []string
	seenURLs := make(map[string]bool)
	for _, u := range startingURLs {
		if u != "" && !seenURLs[u] {
			seenURLs[u] = true
			uniqueURLs = append(uniqueURLs, u)
		}
	}

	for _, u := range uniqueURLs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		item := &types.Item{
			ID:        generateID(),
			URL:       u,
			Protected: protectedURLs[u],
			Render:    renderURLs[u],
		}
		err := ProcessItem(ctx, minion, item, runCtx, "cron")
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

			reportText := runCtx.Stats.GenerateReport(minion.Name)
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
func ProcessItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext, parentName string) error {
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
			var keepWords []string
			if filterMap, ok := val.(map[string]interface{}); ok {
				if dw, ok := filterMap["drop"].([]interface{}); ok {
					for _, w := range dw {
						dropWords = append(dropWords, strings.ToLower(fmt.Sprintf("%v", w)))
					}
				}

				if kw, ok := filterMap["keep"].([]interface{}); ok {
					for _, w := range kw {
						keepWords = append(keepWords, strings.ToLower(fmt.Sprintf("%v", w)))
					}
				}
			}

			var nextArray []types.Item
			for _, m := range matchArray {
				dropped := false
				content := strings.ToLower(fmt.Sprintf("%s %s %s %s", m.URL, m.Text, m.Title, m.Summary))
				
				// 1. Drop check takes precedence
				for _, word := range dropWords {
					if strings.Contains(content, word) {
						step("FILTERED", fmt.Sprintf("Item dropped due to 'drop' keyword: '%s'", word), false)
						dropped = true
						break
					}
				}
				
				// 2. Keep check (only evaluate if not already dropped and keep words exist)
				if !dropped && len(keepWords) > 0 {
					kept := false
					for _, word := range keepWords {
						if strings.Contains(content, word) {
							kept = true
							break
						}
					}
					if !kept {
						step("FILTERED", "Item dropped, did not match any 'keep' keywords", false)
						dropped = true
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

		if scrapeVal, ok := action["scrape"]; ok {
			// timeoutSec is now inherited globally or locally, but we need it locally for HTTP fallback
			timeoutSec := 30
			delaySec := 2 // Default to 2 seconds of max jitter protection
			if scrapeMap, ok := scrapeVal.(map[string]interface{}); ok {
				if t, ok := scrapeMap["timeout"].(int); ok {
					if t > 0 {
						timeoutSec = t
					}
				}
				if d, ok := scrapeMap["delay"].(int); ok {
					delaySec = d
				}
			}

			var nextArray []types.Item
			for _, m := range matchArray {
				if m.URL == "" { continue }

				isDiscarded, err := runCtx.Store.IsDiscarded(m.URL, minion.Filename)
				if err == nil && isDiscarded {
					step("FIREWALL", fmt.Sprintf("Skipping discarded URL: %s", m.URL), false)
					continue
				}

				if m.Text != "" && m.TempHash != "" {
					step("SCRAPE", fmt.Sprintf("Using passed content for %s", m.URL), false)
					
					// Even if passed down, check if the hash has already been processed by *this* minion
					savedHash, _ := runCtx.Store.GetPageHash(m.URL, minion.Filename)
					if savedHash == m.TempHash {
						runCtx.Stats.PagesCached++
						step("CACHED", "Page content unchanged, skipping LLM", false)
						continue
					}
					
					_ = runCtx.Store.UpdatePageHash(m.URL, minion.Filename, m.TempHash)
					
					nextArray = append(nextArray, m)
					continue
				}

				if delaySec > 0 {
					// Add random jitter to prevent bot detection.
					// If delay is 2, rand is 0-1, so sleep is 1-2s.
					// If delay is 5, rand is 0-4, so sleep is 1-5s.
					jitter := rand.Intn(delaySec) + 1
					time.Sleep(time.Duration(jitter) * time.Second)
				}

				step("SCRAPE", m.URL, false)
				var text, hash string
				var fetchErr error
				if m.Render {
					browser, bErr := runCtx.GetBrowser(timeoutSec)
					if bErr != nil {
						runCtx.Stats.Errors++
						step("SCRAPE ERROR", bErr.Error(), true)
						continue
					}
					text, hash, fetchErr = scraper.FetchRenderedAndSanitize(browser, m.URL, timeoutSec)
				} else {
					text, hash, fetchErr = scraper.FetchAndSanitize(m.URL, timeoutSec)
				}
				if fetchErr != nil {
					runCtx.Stats.Errors++
					step("SCRAPE ERROR", fetchErr.Error(), true)
					continue
				}

				runCtx.Stats.PagesScraped++

				savedHash, _ := runCtx.Store.GetPageHash(m.URL, minion.Filename)
				if savedHash == hash {
					runCtx.Stats.PagesCached++
					step("CACHED", "Page content unchanged, skipping LLM", false)
					continue
				}

				// Immedately save the new hash so we don't scrape this again, 
				// even if the LLM drops it or finds 0 matches downstream.
				_ = runCtx.Store.UpdatePageHash(m.URL, minion.Filename, hash)

				m.Text = text
				m.TempHash = hash
				nextArray = append(nextArray, m)
			}
			matchArray = nextArray
			if len(matchArray) == 0 { return nil }
			continue
		}

		if studyVal, ok := action["study"]; ok {
			task := ""
			format := "json_list"
			
			if studyMap, ok := studyVal.(map[string]interface{}); ok {
				if t, ok := studyMap["task"].(string); ok {
					task = t
				}
				if f, ok := studyMap["format"].(string); ok {
					format = f
				}
			}

			var nextArray []types.Item
			for _, m := range matchArray {
				content := m.Text
				if content == "" {
					content = "URL: " + m.URL
				}

				step("STUDY", fmt.Sprintf("Reading %s", m.URL), false)
				runCtx.Stats.PagesStudied++
				
				evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
				res, cost, err := runCtx.LLM.EvaluateText(evalCtx, content, task, format, m.URL)
				evalCancel()

				if err != nil {
					runCtx.Stats.Errors++
					step("STUDY ERROR", fmt.Sprintf("LLM failed for %s: %v", m.URL, err), true)
					continue
				}
				
				runCtx.Stats.TotalCost += cost

				if res.CacheAction == "discard" {
					if m.Protected {
						runCtx.Stats.PagesSkipped++
						step("OVERRIDE", fmt.Sprintf("AI marked %s as discard, but URL is protected. Skipping instead.", m.URL), false)
					} else {
						runCtx.Stats.PagesDiscarded++
						step("DISCARDED", fmt.Sprintf("AI marked %s as irrelevant", m.URL), false)
						_ = runCtx.Store.MarkDiscarded(m.URL, minion.Filename)
					}
				} else if len(res.Matches) == 0 {
					runCtx.Stats.PagesSkipped++
					step("SKIPPED", fmt.Sprintf("Valid page, but 0 items found for %s", m.URL), false)
				}

				runCtx.Stats.ItemsFound += len(res.Matches)

				for _, aiMatch := range res.Matches {
					itemURL := aiMatch.URL
					if itemURL == "" {
						itemURL = m.URL // Inherit the parent's URL if the AI left it blank
					}

					itemURL = strings.TrimSuffix(itemURL, "/")
					cleanParentURL := strings.TrimSuffix(m.URL, "/")

					inheritedText := ""
					inheritedHash := ""
					// ONLY pass the HTML down the chain if the URL is exactly the same!
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

var envRegex = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

func strictExpandEnv(s string) string {
	return envRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		if val, exists := os.LookupEnv(varName); exists {
			return val
		}
		return match
	})
}

func deliverTargets(ctx context.Context, minion *config.MinionConfig, runCtx *RunContext, matchArray []types.Item, targets []map[string]interface{}, saveHash bool) {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	for _, m := range matchArray {
		isDeepLink := m.ParentURL != "" && m.URL != m.ParentURL

		if isDeepLink {
			if runCtx.SmartSplit == nil {
				runCtx.SmartSplit = make(map[string]string)
			}

			if existingID, exists := runCtx.SmartSplit[m.URL]; exists {
				if existingID != m.ID {
					step("DEDUPE", fmt.Sprintf("Smart Split dropped duplicate URL: %s", m.URL), false)
					continue
				}
			} else {
				runCtx.SmartSplit[m.URL] = m.ID
			}
		}

		if saveHash {
			step("ITEM", fmt.Sprintf("%s: %s", m.Title, m.Summary), false)
		}

			for _, t := range targets {
				if ntfyURL, ok := t["ntfy"]; ok {
					urlStr := strictExpandEnv(fmt.Sprintf("%v", ntfyURL))
					var auth *delivery.BasicAuth
					if baData, ok := t["basic_auth"]; ok {
						b, _ := yaml.Marshal(baData)
						yaml.Unmarshal(b, &auth)
						if auth != nil {
							if auth.Username != "" { auth.Username = strictExpandEnv(auth.Username) }
							if auth.Password != "" { auth.Password = strictExpandEnv(auth.Password) }
						}
					}
					
					var useMarkdown bool
					if md, ok := t["markdown"].(bool); ok {
						useMarkdown = md
					}
					
					err := delivery.SendNtfy(urlStr, auth, minion.Name, &m, useMarkdown)
					if err != nil {
						runCtx.Stats.Errors++
						step("DELIVERY ERROR", fmt.Sprintf("ntfy: %v", err), true)
					} else {
						if saveHash { runCtx.Stats.ItemsDelivered++ }
						step("DELIVERY", fmt.Sprintf("Sent ntfy alert for '%s'", m.Title), false)
					}
				}

				if discordURL, ok := t["discord"]; ok {
					urlStr := strictExpandEnv(fmt.Sprintf("%v", discordURL))
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
					urlStr := strictExpandEnv(fmt.Sprintf("%v", reqURL))
					wbConfig := &delivery.HTTPRequestConfig{URL: urlStr}
					
					if method, ok := t["method"].(string); ok {
						wbConfig.Method = method
					}
					if tmpl, ok := t["payload_template"].(string); ok {
						wbConfig.PayloadTemplate = tmpl // template has its own {{.}} variable structure, no strictExpandEnv here
					}
					if headers, ok := t["headers"].(map[string]interface{}); ok {
						wbConfig.Headers = make(map[string]string)
						for k, v := range headers {
							wbConfig.Headers[k] = strictExpandEnv(fmt.Sprintf("%v", v))
						}
					}
					if baData, ok := t["basic_auth"]; ok {
						b, _ := yaml.Marshal(baData)
						var ba delivery.BasicAuth
						yaml.Unmarshal(b, &ba)
						if ba.Username != "" { ba.Username = strictExpandEnv(ba.Username) }
						if ba.Password != "" { ba.Password = strictExpandEnv(ba.Password) }
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
					err = ProcessItem(ctx, targetMinion, &m, runCtx, minion.Filename)
					if err != nil {
						runCtx.Stats.Errors++
						step("CHAIN ERROR", fmt.Sprintf("Target minion '%s' failed: %v", targetMinionName, err), true)
					} else {
						if saveHash { runCtx.Stats.ItemsDelivered++ }
					}
				}
			}
		}
	}
}

func generateID() string {
	b := make([]byte, 8)
	crand.Read(b)
	return hex.EncodeToString(b)
}
