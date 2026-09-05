package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"minion/internal/config"
	"minion/internal/store"
	"minion/internal/types"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.InitStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func setupTestRunCtx(s *store.Store) *RunContext {
	return &RunContext{
		Store: s,
		Stats: &Stats{},
	}
}

func TestProcessDoOnly_EmptyDo(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
	}

	err := processDoOnly(context.Background(), m, &types.Item{SourceType: "do"}, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestProcessDoOnly_NoModel(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "find me something",
	}

	os.Clearenv()
	err := processDoOnly(context.Background(), m, &types.Item{SourceType: "do"}, runCtx)
	if err != nil {
		t.Fatalf("expected nil (model error is non-fatal), got %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (missing model), got %d", runCtx.Stats.Errors)
	}
	if runCtx.Stats.Analyzed != 1 {
		t.Fatalf("expected 1 analyzed, got %d", runCtx.Stats.Analyzed)
	}
}

func TestProcessDoOnly_LLMError(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "find me something",
		Settings: config.Settings{Model: "test-model"},
	}

	err := processDoOnly(context.Background(), m, &types.Item{SourceType: "do"}, runCtx)
	if err != nil {
		t.Fatalf("expected nil error (LLM errors are non-fatal), got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", runCtx.Stats.Errors)
	}
}

func TestProcessDoOnly_WithTellButNoMatches(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "find me something",
		Settings: config.Settings{Model: "test-model"},
		Tell:     []map[string]interface{}{{"ntfy": "http://example.com"}},
	}

	err := processDoOnly(context.Background(), m, &types.Item{SourceType: "do"}, runCtx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (LLM failure), got %d", runCtx.Stats.Errors)
	}
}

func TestProcessURLItem_NoModel(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "extract something",
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "", // empty URL, passes fetch, goes to Do phase
	}

	os.Clearenv()
	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil (model error is non-fatal), got %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (missing model), got %d", runCtx.Stats.Errors)
	}
	if runCtx.Stats.Analyzed != 1 {
		t.Fatalf("expected 1 analyzed, got %d", runCtx.Stats.Analyzed)
	}
}

func TestProcessURLItem_EmptyURL(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_LLMError(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "extract something",
		Settings: config.Settings{Model: "test-model"},
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil (LLM errors non-fatal), got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", runCtx.Stats.Errors)
	}
}

func TestProcessURLItem_Discarded(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
	}

	err := s.MarkDiscarded("https://example.com", "test.yaml")
	if err != nil {
		t.Fatalf("failed to mark discarded: %v", err)
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "https://example.com",
	}

	err = processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_PassedThroughUnchanged(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
	}

	hash := "test-hash-123"
	_ = s.UpdatePageHash("https://example.com", "test.yaml", hash)

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "https://example.com",
		Text:       "some text",
		TempHash:   hash,
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if runCtx.Stats.Unchanged != 1 {
		t.Fatalf("expected 1 unchanged, got %d", runCtx.Stats.Unchanged)
	}
}

func TestProcessURLItem_PassedThroughChanged(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "extract something",
		Settings: config.Settings{Model: "test-model"},
	}

	_ = s.UpdatePageHash("https://example.com", "test.yaml", "old-hash")

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "https://example.com",
		Text:       "some text",
		TempHash:   "new-hash",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if runCtx.Stats.Unchanged != 0 {
		t.Fatalf("expected 0 unchanged, got %d", runCtx.Stats.Unchanged)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (from LLM), got %d", runCtx.Stats.Errors)
	}
}

func TestProcessURLItem_KeepFilter(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Keep:     []string{"important"},
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		Text:       "this is important content",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_KeepFilterDrops(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Keep:     []string{"important"},
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		Text:       "this is not relevant",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_IgnoreFilter(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Ignore:   []string{"spam"},
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		Text:       "this is spam content",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_NoDoNoTell(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		Text:       "some content here",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_EmitsResultWithoutTell(t *testing.T) {
	var results []types.Item
	runCtx := &RunContext{
		Stats:     &Stats{},
		Ephemeral: true,
		OnResult: func(item types.Item) {
			results = append(results, item)
		},
	}
	m := &config.MinionConfig{Name: "inline", Filename: "inline"}
	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "https://example.com",
		Text:       "sanitized content",
		TempHash:   "content-hash",
	}

	if err := processURLItem(context.Background(), m, item, runCtx); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Text != "sanitized content" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestProcessURLItem_EphemeralBypassesPersistentState(t *testing.T) {
	s := setupTestStore(t)
	if err := s.UpdatePageHash("https://example.com", "inline", "old-hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDiscarded("https://example.com", "inline"); err != nil {
		t.Fatal(err)
	}

	var results []types.Item
	runCtx := &RunContext{
		Store:     s,
		Stats:     &Stats{},
		Ephemeral: true,
		OnResult: func(item types.Item) {
			results = append(results, item)
		},
	}
	m := &config.MinionConfig{Name: "inline", Filename: "inline"}
	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "https://example.com",
		Text:       "content",
		TempHash:   "new-hash",
	}

	if err := processURLItem(context.Background(), m, item, runCtx); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected discarded and previously seen item to be emitted, got %d results", len(results))
	}
	if runCtx.Stats.Unchanged != 0 {
		t.Fatalf("expected no persistent dedup, got %d unchanged", runCtx.Stats.Unchanged)
	}
	hash, err := s.GetPageHash("https://example.com", "inline")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "old-hash" {
		t.Fatalf("ephemeral run mutated stored hash: %q", hash)
	}
}

func TestRunMission_EphemeralFetchesSanitizedContentWithoutStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body><script>hidden()</script><main>Visible content <a href="/docs">Docs</a></main></body></html>`)
	}))
	defer server.Close()

	var results []types.Item
	runCtx := &RunContext{
		Ephemeral: true,
		OnResult: func(item types.Item) {
			results = append(results, item)
		},
	}
	m := &config.MinionConfig{
		Name:     "inline",
		Filename: "inline",
		From:     []config.Source{{URL: server.URL}},
		Settings: config.Settings{Timeout: 5, Delay: 1},
	}

	if err := RunMission(context.Background(), m, runCtx); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one raw result, got %d", len(results))
	}
	if !strings.Contains(results[0].Text, "Visible content") || !strings.Contains(results[0].Text, "[Link: "+server.URL+"/docs]") {
		t.Fatalf("expected sanitized page content, got %q", results[0].Text)
	}
	if strings.Contains(results[0].Text, "hidden") {
		t.Fatalf("script content was not sanitized: %q", results[0].Text)
	}
}

func TestProcessFileItem_EphemeralIgnoresCursor(t *testing.T) {
	s := setupTestStore(t)
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("first\n---\nsecond\n"), 0644); err != nil {
		t.Fatal(err)
	}
	firstHash := sha256.Sum256([]byte("first"))
	cursor := fmt.Sprintf("%x", firstHash[:8])
	if err := s.UpdateFileHash(path, "inline", cursor); err != nil {
		t.Fatal(err)
	}

	var results []types.Item
	runCtx := &RunContext{
		Store:     s,
		Stats:     &Stats{},
		Ephemeral: true,
		OnResult: func(item types.Item) {
			results = append(results, item)
		},
	}
	m := &config.MinionConfig{Name: "inline", Filename: "inline"}

	if err := processFileItem(context.Background(), m, &types.Item{FilePath: path}, runCtx); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Text != "first" || results[1].Text != "second" {
		t.Fatalf("expected every file record, got %#v", results)
	}
	savedCursor, err := s.GetFileHash(path, "inline")
	if err != nil {
		t.Fatal(err)
	}
	if savedCursor != cursor {
		t.Fatalf("ephemeral run mutated file cursor: %q", savedCursor)
	}
}

func TestRunMission_EphemeralDoesNotRequireStore(t *testing.T) {
	runCtx := &RunContext{Ephemeral: true}
	m := &config.MinionConfig{Name: "inline", Filename: "inline"}

	if err := RunMission(context.Background(), m, runCtx); err != nil {
		t.Fatal(err)
	}
	if runCtx.Stats == nil {
		t.Fatal("expected mission stats")
	}
}

func TestProcessMinionChain_NoModel(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		Do:       "extract something",
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		URL:        "https://example.com",
	}

	os.Clearenv()
	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil (model error non-fatal), got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (missing model), got %d", runCtx.Stats.Errors)
	}
}

func TestProcessMinionChain_EmptyItem(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessMinionChain_Discarded(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
	}

	_ = s.MarkDiscarded("https://example.com", "target.yaml")

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		URL:        "https://example.com",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessMinionChain_Unchanged(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
	}

	hash := "same-hash"
	_ = s.UpdatePageHash("https://example.com", "target.yaml", hash)

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		URL:        "https://example.com",
		Text:       "content",
		TempHash:   hash,
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if runCtx.Stats.Unchanged != 1 {
		t.Fatalf("expected 1 unchanged, got %d", runCtx.Stats.Unchanged)
	}
}

func TestProcessMinionChain_LLMError(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		Do:       "extract something",
		Settings: config.Settings{Model: "test-model"},
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		Text:       "some inherited content",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil (LLM error non-fatal), got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", runCtx.Stats.Errors)
	}
}

func TestProcessMinionChain_KeepFilter(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		Keep:     []string{"urgent"},
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		Text:       "this is urgent content",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessMinionChain_Authorized_WithExtensionMatch(t *testing.T) {
	// When parentName has .yaml but source.Minion does not, should match after fix
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		From:     []config.Source{{Minion: "daddy"}}, // no .yaml
		Do:       "extract something",
		Settings: config.Settings{Model: "test-model"},
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		Text:       "some content",
	}

	// parentName "daddy.yaml" should match source.Minion "daddy" after normalization
	err := processMinionChain(context.Background(), m, item, runCtx, "daddy.yaml")
	if err != nil {
		t.Fatalf("expected nil (authorized should pass), got: %v", err)
	}
}

func TestProcessMinionChain_Authorized_WithExactMatch(t *testing.T) {
	// When both parentName and source.Minion are without .yaml, should still match
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		From:     []config.Source{{Minion: "daddy"}}, // no .yaml
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "daddy")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessMinionChain_Authorized_WithBothExtensions(t *testing.T) {
	// When both have .yaml, should still match
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		From:     []config.Source{{Minion: "daddy.yaml"}}, // with .yaml
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "daddy.yaml")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessMinionChain_Unauthorized(t *testing.T) {
	// Different parentName should be rejected
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		From:     []config.Source{{Minion: "allowed_parent"}},
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "unknown_parent")
	if err != nil {
		t.Fatalf("expected nil (rejection is non-fatal), got: %v", err)
	}
}

func TestProcessMinionChain_Authorized_WithYmlExtension(t *testing.T) {
	// .yml extension should also be stripped
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		From:     []config.Source{{Minion: "daddy"}},
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "daddy.yml")
	if err != nil {
		t.Fatalf("expected nil (.yml should be stripped), got: %v", err)
	}
}

func TestProcessItem_Dispatcher(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)

	tests := []struct {
		name       string
		minion     *config.MinionConfig
		item       *types.Item
		wantErrors int
	}{
		{
			name: "do source type no Do",
			minion: &config.MinionConfig{
				Name:     "test",
				Filename: "test.yaml",
			},
			item: &types.Item{
				ID:         "do-id",
				SourceType: "do",
			},
			wantErrors: 0,
		},
		{
			name: "do source type with Do and model",
			minion: &config.MinionConfig{
				Name:     "test",
				Filename: "test.yaml",
				Do:       "do something",
				Settings: config.Settings{Model: "test-model"},
			},
			item: &types.Item{
				ID:         "do-id-2",
				SourceType: "do",
			},
			wantErrors: 1, // LLM call will fail
		},
		{
			name: "minion source type with Do and model",
			minion: &config.MinionConfig{
				Name:     "target",
				Filename: "target.yaml",
				From:     []config.Source{{Minion: "parent"}},
				Do:       "extract something",
				Settings: config.Settings{Model: "test-model"},
			},
			item: &types.Item{
				ID:         "minion-id",
				SourceType: "minion",
				Text:       "some content",
			},
			wantErrors: 1, // LLM call will fail
		},
		{
			name: "url source type with no Do",
			minion: &config.MinionConfig{
				Name:     "test",
				Filename: "test.yaml",
			},
			item: &types.Item{
				ID:         "url-id",
				SourceType: "url",
			},
			wantErrors: 0,
		},
		{
			name: "url source type with Do and model",
			minion: &config.MinionConfig{
				Name:     "test",
				Filename: "test.yaml",
				Do:       "extract",
				Settings: config.Settings{Model: "test-model"},
			},
			item: &types.Item{
				ID:         "url-id-2",
				SourceType: "url",
				URL:        "",
			},
			wantErrors: 1, // LLM call will fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCtx.Stats = &Stats{}
			err := ProcessItem(context.Background(), tt.minion, tt.item, runCtx, "parent")
			if err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if runCtx.Stats.Errors != tt.wantErrors {
				t.Fatalf("expected %d errors, got %d", tt.wantErrors, runCtx.Stats.Errors)
			}
		})
	}
}

func TestRunMission_NoSourcesWithDo(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "do something",
		Settings: config.Settings{Model: "test-model"},
	}

	err := RunMission(context.Background(), m, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (LLM failure), got %d", runCtx.Stats.Errors)
	}
	if runCtx.Stats.Analyzed != 1 {
		t.Fatalf("expected 1 analyzed, got %d", runCtx.Stats.Analyzed)
	}
}

func TestRunMission_NoSourcesNoDo(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
	}

	err := RunMission(context.Background(), m, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if runCtx.Stats.Errors != 0 {
		t.Fatalf("expected 0 errors, got %d", runCtx.Stats.Errors)
	}
}

func TestRunMission_WithURLSource(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		From: []config.Source{
			{URL: "https://example.com"},
		},
		Settings: config.Settings{Model: "test-model"},
		Do:       "extract something",
	}

	err := RunMission(context.Background(), m, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	t.Logf("Stats: Fetched=%d Unchanged=%d Analyzed=%d Discarded=%d Skipped=%d Results=%d Sent=%d Errors=%d",
		runCtx.Stats.Fetched, runCtx.Stats.Unchanged, runCtx.Stats.Analyzed,
		runCtx.Stats.Discarded, runCtx.Stats.Skipped, runCtx.Stats.Results,
		runCtx.Stats.Sent, runCtx.Stats.Errors)
}

func TestRunMission_DedupURLs(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		From: []config.Source{
			{URL: "https://example.com"},
			{URL: "https://example.com"},
			{URL: "https://example.org"},
		},
		Settings: config.Settings{Model: "test-model"},
	}

	err := RunMission(context.Background(), m, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}

	// 2 unique URLs, both should be processed (fetches may succeed or fail)
	t.Logf("Stats: Fetched=%d Errors=%d", runCtx.Stats.Fetched, runCtx.Stats.Errors)
}

func TestModelResolution_EnvVar(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "test",
	}

	os.Clearenv()
	t.Setenv("DEFAULT_MODEL", "env-model")

	err := processDoOnly(context.Background(), m, &types.Item{SourceType: "do"}, runCtx)
	if err != nil {
		t.Fatalf("expected nil (LLM error non-fatal), got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (LLM failure), got %d", runCtx.Stats.Errors)
	}
}

func TestModelResolution_SettingTakesPrecedence(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "test",
		Settings: config.Settings{Model: "settings-model"},
	}

	t.Setenv("DEFAULT_MODEL", "env-model")

	err := processDoOnly(context.Background(), m, &types.Item{SourceType: "do"}, runCtx)
	if err != nil {
		t.Fatalf("expected nil (LLM error non-fatal), got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 error (LLM failure), got %d", runCtx.Stats.Errors)
	}
}

func TestRunMission_ReportTarget(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Do:       "do something",
		Settings: config.Settings{Model: "test-model"},
		Report:   []map[string]interface{}{{"ntfy": "http://example.com"}},
	}

	err := RunMission(context.Background(), m, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessURLItem_NoFetchWithDelay(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Settings: config.Settings{Delay: 0},
	}

	item := &types.Item{
		ID:         "test-id",
		SourceType: "url",
		URL:        "https://example.com",
	}

	err := processURLItem(context.Background(), m, item, runCtx)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestProcessMinionChain_WithURL(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "target",
		Filename: "target.yaml",
		Do:       "extract something",
		Settings: config.Settings{Model: "test-model"},
	}

	item := &types.Item{
		ID:         "chain-id",
		SourceType: "minion",
		URL:        "https://example.com",
		Text:       "some inherited page content",
		TempHash:   "new-hash",
	}

	err := processMinionChain(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected 1 LLM error, got %d", runCtx.Stats.Errors)
	}
}

func TestStatsCounters_ProcessURLItem(t *testing.T) {
	s := setupTestStore(t)
	runCtx := setupTestRunCtx(s)
	m := &config.MinionConfig{
		Name:     "test",
		Filename: "test.yaml",
		Settings: config.Settings{Model: "test-model"},
		Do:       "extract items",
	}

	item := &types.Item{
		ID:         "id1",
		SourceType: "url",
		URL:        "https://example.com/page1",
	}
	item2 := &types.Item{
		ID:         "id2",
		SourceType: "url",
		URL:        "https://example.com/page2",
	}
	_ = s.MarkDiscarded("https://example.com/page2", "test.yaml")

	err := ProcessItem(context.Background(), m, item, runCtx, "cron")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	err2 := ProcessItem(context.Background(), m, item2, runCtx, "cron")
	if err2 != nil {
		t.Fatalf("expected nil, got: %v", err2)
	}

	t.Logf("Stats: Fetched=%d Unchanged=%d Analyzed=%d Discarded=%d Skipped=%d Results=%d Sent=%d Errors=%d",
		runCtx.Stats.Fetched, runCtx.Stats.Unchanged, runCtx.Stats.Analyzed,
		runCtx.Stats.Discarded, runCtx.Stats.Skipped, runCtx.Stats.Results,
		runCtx.Stats.Sent, runCtx.Stats.Errors)
}

func TestBuildURLResultItems_NoText(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page"}
	matches := []urlMatch{
		{Title: "t1", URL: "https://example.com/page", Summary: "s1"},
		{Title: "t2", URL: "", Summary: "s2"},
	}
	items := buildURLResultItems(parent, matches)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for i, item := range items {
		if item.Text != "" {
			t.Fatalf("item[%d].Text should be empty after do step, got %q", i, item.Text)
		}
		if item.TempHash != "" {
			t.Fatalf("item[%d].TempHash should be empty after do step, got %q", i, item.TempHash)
		}
	}
}

func TestBuildURLResultItems_URLFallback(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page"}
	matches := []urlMatch{
		{Title: "t1", URL: "", Summary: "s1"},
	}
	items := buildURLResultItems(parent, matches)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].URL != "https://example.com/page" {
		t.Fatalf("expected parent URL fallback, got %q", items[0].URL)
	}
}

func TestBuildURLResultItems_TrailingSlash(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page/"}
	matches := []urlMatch{
		{Title: "t1", URL: "https://example.com/other/", Summary: "s1"},
	}
	items := buildURLResultItems(parent, matches)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].URL != "https://example.com/other" {
		t.Fatalf("expected trailing slash stripped, got %q", items[0].URL)
	}
	if items[0].ParentURL != "https://example.com/page" {
		t.Fatalf("expected parent trailing slash stripped, got %q", items[0].ParentURL)
	}
}

func TestBuildMinionResultItems_NoText(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page"}
	matches := []minionMatch{
		{Title: "t1", URL: "https://example.com/page", Summary: "s1"},
		{Title: "t2", URL: "", Summary: "s2"},
	}
	items := buildMinionResultItems(parent, matches)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for i, item := range items {
		if item.Text != "" {
			t.Fatalf("item[%d].Text should be empty after do step, got %q", i, item.Text)
		}
		if item.TempHash != "" {
			t.Fatalf("item[%d].TempHash should be empty after do step, got %q", i, item.TempHash)
		}
	}
}

func TestBuildMinionResultItems_URLFallback(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page"}
	matches := []minionMatch{
		{Title: "t1", URL: "", Summary: "s1"},
	}
	items := buildMinionResultItems(parent, matches)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].URL != "https://example.com/page" {
		t.Fatalf("expected parent URL fallback, got %q", items[0].URL)
	}
}

func TestBuildFileResultItems_NoText(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page"}
	matches := []fileMatch{
		{Title: "t1", URL: "https://example.com/page", Summary: "s1"},
		{Title: "t2", URL: "", Summary: "s2"},
	}
	items := buildFileResultItems(parent, matches)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for i, item := range items {
		if item.Text != "" {
			t.Fatalf("item[%d].Text should be empty after do step, got %q", i, item.Text)
		}
		if item.TempHash != "" {
			t.Fatalf("item[%d].TempHash should be empty after do step, got %q", i, item.TempHash)
		}
	}
}

func TestBuildCommandResultItems_NoText(t *testing.T) {
	matches := []commandMatch{
		{Title: "t1", URL: "https://example.com", Summary: "s1"},
	}
	items := buildCommandResultItems(matches)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Text != "" {
		t.Fatalf("item.Text should be empty after do step, got %q", items[0].Text)
	}
	if items[0].TempHash != "" {
		t.Fatalf("item.TempHash should be empty after do step, got %q", items[0].TempHash)
	}
}

func TestBuildDoResultItems_NoText(t *testing.T) {
	matches := []doMatch{
		{Title: "t1", URL: "https://example.com", Summary: "s1"},
	}
	items := buildDoResultItems(matches)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Text != "" {
		t.Fatalf("item.Text should be empty after do step, got %q", items[0].Text)
	}
	if items[0].TempHash != "" {
		t.Fatalf("item.TempHash should be empty after do step, got %q", items[0].TempHash)
	}
}

func TestBuilder_TitleSummaryPassthrough(t *testing.T) {
	parent := types.Item{URL: "https://example.com/page"}
	matches := []urlMatch{
		{Title: "my title", Summary: "my summary"},
	}
	items := buildURLResultItems(parent, matches)
	if items[0].Title != "my title" {
		t.Fatalf("expected title 'my title', got %q", items[0].Title)
	}
	if items[0].Summary != "my summary" {
		t.Fatalf("expected summary 'my summary', got %q", items[0].Summary)
	}
}

func TestLLMParseStripping(t *testing.T) {
	// Test the JSON stripping logic that exists in all from_*.go files
	raw := "```json\n{\n  \"matches\": [{ \"title\": \"test\", \"url\": \"\", \"summary\": \"test item\" }]\n}\n```"

	parsed := raw
	parsed = strings.TrimSpace(parsed)
	if strings.HasPrefix(parsed, "```json") {
		parsed = strings.TrimPrefix(parsed, "```json")
	} else if strings.HasPrefix(parsed, "```") {
		parsed = strings.TrimPrefix(parsed, "```")
	}
	if strings.HasSuffix(parsed, "```") {
		parsed = strings.TrimSuffix(parsed, "```")
	}

	startIdx := strings.Index(parsed, "{")
	endIdx := strings.LastIndex(parsed, "}")
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		parsed = parsed[startIdx : endIdx+1]
	}
	parsed = strings.TrimSpace(parsed)

	// Should preserve internal JSON formatting
	if !strings.Contains(parsed, `"test"`) {
		t.Fatalf("expected test content in parsed output: %s", parsed)
	}
	if !strings.Contains(parsed, `"test item"`) {
		t.Fatalf("expected test item in parsed output: %s", parsed)
	}
}
