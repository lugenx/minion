package engine

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	"minion/internal/config"
	"minion/internal/delivery"
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
	hasNonMinionSource := false

	rand.Seed(time.Now().UnixNano())

	for _, source := range minion.From {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if source.Minion != "" {
			continue
		}

		hasNonMinionSource = true

		if source.Search != "" {
			if len(startingURLs) > 0 {
				time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
			}
			urls := gatherFromSearchSource(ctx, minion, &source, runCtx)
			if urls != nil {
				startingURLs = append(startingURLs, urls...)
			}
			continue
		}

		if source.Command != "" {
			step("from", fmt.Sprintf("command `%s`", source.Command), false)
			err := ProcessItem(ctx, minion, &types.Item{
				ID:         generateID(),
				SourceType: "command",
				Command:    source.Command,
			}, runCtx, "cron")
			if err != nil {
				runCtx.Stats.Errors++
				step("error", err.Error(), true)
			}
			continue
		}

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

	if !hasNonMinionSource {
		step("", "", false)
		err := ProcessItem(ctx, minion, &types.Item{
			ID:         generateID(),
			SourceType: "do",
		}, runCtx, "cron")
		if err != nil {
			runCtx.Stats.Errors++
			step("error", err.Error(), true)
		}
	} else {
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

			step("", "", false)

			item := &types.Item{
				ID:         generateID(),
				URL:        u,
				Protected:  protectedURLs[u],
				Render:     renderURLs[u],
				SourceType: "url",
			}
			err := ProcessItem(ctx, minion, item, runCtx, "cron")
			if err != nil {
				runCtx.Stats.Errors++
				step("error", fmt.Sprintf("→ `%s`: %v", u, err), true)
			}
		}
	}

	runCtx.Stats.EndTime = time.Now()

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

func ProcessItem(ctx context.Context, minion *config.MinionConfig, item *types.Item, runCtx *RunContext, parentName string) error {
	switch item.SourceType {
	case "do":
		return processDoOnly(ctx, minion, item, runCtx)
	case "minion":
		return processMinionChain(ctx, minion, item, runCtx, parentName)
	case "command":
		return processCommandItem(ctx, minion, item, runCtx)
	default:
		return processURLItem(ctx, minion, item, runCtx)
	}
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
					wbConfig.PayloadTemplate = tmpl
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
					chainItem := m
					chainItem.SourceType = "minion"
					err = ProcessItem(ctx, targetMinion, &chainItem, runCtx, minion.Filename)
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
