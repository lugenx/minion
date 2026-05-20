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
	"minion/internal/delivery"
	"minion/internal/llm"
	"minion/internal/scraper"
	"minion/internal/store"
	"minion/internal/types"
)

type Stats struct {
	StartTime  time.Time
	EndTime    time.Time
	Fetched    int
	Unchanged  int
	Analyzed   int
	Discarded  int
	Skipped    int
	Results    int
	Sent       int
	Errors     int
	TotalCost  float64
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
			"  Fetched:      %d\n"+
			"  Unchanged:    %d\n"+
			"  Analyzed:     %d\n"+
			"  Discarded:    %d\n"+
			"  Skipped:      %d\n\n"+
			"  Results:      %d\n"+
			"  Sent:         %d\n\n"+
			"Cost:   %s",
		minionName,
		s.StartTime.Format("2006-01-02 15:04:05"),
		s.EndTime.Format("15:04:05"),
		s.Duration().Round(time.Millisecond*100),
		s.Errors,
		s.Fetched, s.Unchanged,
		s.Analyzed, s.Discarded, s.Skipped,
		s.Results, s.Sent,
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

	step("start", minion.Name, false)

	if runCtx.Stats == nil {
		runCtx.Stats = &Stats{StartTime: time.Now()}
	}

	globalTimeout := minion.Settings.Timeout
	if globalTimeout <= 0 {
		globalTimeout = 30
	}

	var startingURLs []string
	protectedURLs := make(map[string]bool)
	renderURLs := make(map[string]bool)

	// 1. GENERATORS (Gather all URLs from `from:`)
	rand.Seed(time.Now().UnixNano())

	for _, source := range minion.From {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Skip minion sources (chain authorization, not URL generators)
		if source.Minion != "" {
			continue
		}

		// Handle search
		if source.Search != "" {
			limit := source.Limit
			if limit <= 0 {
				limit = 3
			}

			if len(startingURLs) > 0 {
				time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
			}
			step("from", fmt.Sprintf("search `%s`", source.Search), false)
			urls, err := scraper.SearchDuckDuckGo(ctx, source.Search, limit, 15)
			if err != nil {
				runCtx.Stats.Errors++
				step("from", err.Error(), true)
				continue
			}
			runCtx.Stats.Fetched += len(urls)
			startingURLs = append(startingURLs, urls...)
			continue
		}

		// Handle browse/url
		if source.URL != "" {
			if source.Match == "" {
				startingURLs = append(startingURLs, source.URL)
				protectedURLs[source.URL] = true
				if source.Render {
					renderURLs[source.URL] = true
				}
				runCtx.Stats.Fetched++
				step("from", fmt.Sprintf("url `%s`", source.URL), false)
			} else {
				step("from", fmt.Sprintf("url `%s`", source.URL), false)
				var links []string
				var err error
				if source.Render {
					browser, bErr := runCtx.GetBrowser(globalTimeout)
					if bErr != nil {
						runCtx.Stats.Errors++
						step("from", bErr.Error(), true)
						continue
					}
					links, err = scraper.ExtractLinksRendered(ctx, browser, source.URL, source.Match, globalTimeout)
				} else {
					links, err = scraper.ExtractLinks(ctx, source.URL, source.Match, globalTimeout)
				}
				if err != nil {
					runCtx.Stats.Errors++
					step("from", err.Error(), true)
					continue
				}
				step("from", fmt.Sprintf("follow `%s` → %d urls", source.Match, len(links)), false)
				runCtx.Stats.Fetched += len(links)
				startingURLs = append(startingURLs, links...)
				if source.Render {
					for _, link := range links {
						renderURLs[link] = true
					}
				}
			}
		}
	}

	// 2. THE LINEAR STREAM (Process one by one)
	var uniqueURLs []string
	seenURLs := make(map[string]bool)
	for _, u := range startingURLs {
		if u != "" && !seenURLs[u] {
			seenURLs[u] = true
			uniqueURLs = append(uniqueURLs, u)
		}
	}

	// Always process at least once, even with empty URL (for static study tasks, downstream worker receives)
	if len(uniqueURLs) == 0 {
		uniqueURLs = append(uniqueURLs, "")
	}

	for _, u := range uniqueURLs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		step("", "", false)

		item := &types.Item{
			ID:        generateID(),
			URL:       u,
			Protected: protectedURLs[u],
			Render:    renderURLs[u],
		}
		err := ProcessItem(ctx, minion, item, runCtx, "cron")
		if err != nil {
			runCtx.Stats.Errors++
			step("error", fmt.Sprintf("→ `%s`: %v", u, err), true)
		}
	}

	runCtx.Stats.EndTime = time.Now()

	// 3. FINAL REPORTING
	if len(minion.Report) > 0 {
		reportText := runCtx.Stats.GenerateReport(minion.Name)
		reportItem := types.Item{
			Title:   fmt.Sprintf("Mission Report: %s", minion.Name),
			Summary: reportText,
		}

		step("report", "delivered", false)
		deliverTargets(ctx, minion, runCtx, []types.Item{reportItem}, minion.Report, false)
	}

	step("done", minion.Name, false)
	return nil
}

// ProcessItem pushes a single item through the pipeline (Filter -> Scrape -> Study -> Deliver).
func ProcessItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext, parentName string) error {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	var matchArray []types.Item
	matchArray = append(matchArray, *item)

	// Step 0: Security Checkpoint (from.minion)
	if parentName != "" && parentName != "cron" {
		authorized := false
		for _, source := range minion.From {
			if source.Minion != "" && parentName == source.Minion {
				authorized = true
				break
			}
		}
		if !authorized {
			step("from", fmt.Sprintf("rejected `%s`", parentName), true)
			return nil
		}
		step("from", fmt.Sprintf("from `%s`", parentName), false)
	}

	// Step 1: Scrape
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

	// Step 2: Filter
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

			// 1. Ignore takes precedence
			for _, word := range dropWords {
				if strings.Contains(content, word) {
					step("ignore", fmt.Sprintf("dropped `%s`", word), false)
					dropped = true
					break
				}
			}

			// 2. Keep check
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

	// Step 3: Study (LLM)
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

			evalCtx, evalCancel := context.WithTimeout(ctx, 120*time.Second)
			modelOverride := minion.Settings.Model
			res, cost, err := runCtx.LLM.EvaluateText(evalCtx, content, minion.Do, "json_list", m.URL, modelOverride)
			evalCancel()

			if err != nil {
				runCtx.Stats.Errors++
				step("do", fmt.Sprintf("`%s` → %v", m.URL, err), true)
				continue
			}

			runCtx.Stats.TotalCost += cost

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

	// Step 4: Deliver
	if len(minion.Tell) > 0 {
		deliverTargets(ctx, minion, runCtx, matchArray, minion.Tell, true)
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
		if ctx.Err() != nil {
			return
		}
		isDeepLink := m.ParentURL != "" && m.URL != m.ParentURL

		if isDeepLink {
			if runCtx.SmartSplit == nil {
				runCtx.SmartSplit = make(map[string]string)
			}

			if existingID, exists := runCtx.SmartSplit[m.URL]; exists {
				if existingID != m.ID {
					step("tell", fmt.Sprintf("duplicate: `%s`", m.URL), false)
					continue
				}
			} else {
				runCtx.SmartSplit[m.URL] = m.ID
			}
		}

		if saveHash {
			step("result", fmt.Sprintf("→ %s", m.Title), false)
		}

		for _, t := range targets {
			if ntfyURL, ok := t["ntfy"]; ok {
				urlStr := strictExpandEnv(fmt.Sprintf("%v", ntfyURL))
				var auth *delivery.BasicAuth
				if baData, ok := t["basic_auth"]; ok {
					b, _ := yaml.Marshal(baData)
					yaml.Unmarshal(b, &auth)
					if auth != nil {
						if auth.Username != "" {
							auth.Username = strictExpandEnv(auth.Username)
						}
						if auth.Password != "" {
							auth.Password = strictExpandEnv(auth.Password)
						}
					}
				}

				var useMarkdown bool
				if md, ok := t["markdown"].(bool); ok {
					useMarkdown = md
				}

				err := delivery.SendNtfy(urlStr, auth, minion.Name, &m, useMarkdown)
				if err != nil {
					runCtx.Stats.Errors++
					step("tell", fmt.Sprintf("ntfy: %v", err), true)
				} else {
					if saveHash {
						runCtx.Stats.Sent++
					}
					step("tell", fmt.Sprintf("ntfy → %s", m.Title), false)
				}
			}

			if discordURL, ok := t["discord"]; ok {
				urlStr := strictExpandEnv(fmt.Sprintf("%v", discordURL))
				err := delivery.SendDiscord(urlStr, minion.Name, &m)
				if err != nil {
					runCtx.Stats.Errors++
					step("tell", fmt.Sprintf("discord: %v", err), true)
				} else {
					if saveHash {
						runCtx.Stats.Sent++
					}
					step("tell", fmt.Sprintf("discord → %s", m.Title), false)
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
					if ba.Username != "" {
						ba.Username = strictExpandEnv(ba.Username)
					}
					if ba.Password != "" {
						ba.Password = strictExpandEnv(ba.Password)
					}
					wbConfig.BasicAuth = &ba
				}

				err := delivery.SendHTTPRequest(wbConfig, &m)
				if err != nil {
					runCtx.Stats.Errors++
					step("tell", fmt.Sprintf("http: %v", err), true)
				} else {
					if saveHash {
						runCtx.Stats.Sent++
					}
					step("tell", fmt.Sprintf("http → %s", m.Title), false)
				}
			}

			if minionNameRaw, ok := t["minion"]; ok {
				targetMinionName := fmt.Sprintf("%v", minionNameRaw)
				step("tell", fmt.Sprintf("→ minion `%s`", targetMinionName), false)

				targetMinion, err := config.LoadMinion(targetMinionName)
				if err != nil {
					runCtx.Stats.Errors++
					step("tell", fmt.Sprintf("→ minion `%s`: %v", targetMinionName, err), true)
				} else {
					err = ProcessItem(ctx, targetMinion, &m, runCtx, minion.Filename)
					if err != nil {
						runCtx.Stats.Errors++
step("tell", fmt.Sprintf("→ minion `%s`: %v", targetMinionName, err), true)
					} else {
						if saveHash {
							runCtx.Stats.Sent++
						}
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
