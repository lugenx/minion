package engine

import (
	"context"
	"fmt"

	"minion/internal/config"
	"minion/internal/scraper"
)

func gatherFromSearchSource(ctx context.Context, minion *config.MinionConfig, source *config.Source, runCtx *RunContext) []string {
	step := func(s, details string, isError bool) {
		if runCtx.OnStep != nil {
			runCtx.OnStep(s, details, isError)
		}
	}

	limit := source.Limit
	if limit <= 0 {
		limit = 3
	}

	step("from", fmt.Sprintf("search `%s`", source.Search), false)
	urls, err := scraper.SearchDuckDuckGo(ctx, source.Search, limit, 15)
	if err != nil {
		runCtx.Stats.Errors++
		step("from", err.Error(), true)
		return nil
	}
	runCtx.Stats.Fetched += len(urls)
	return urls
}
