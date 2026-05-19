package scraper

import (
	"context"
	"testing"
	"time"
)

func TestSearchDuckDuckGo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	results, err := SearchDuckDuckGo(ctx, "golang programming", 3, 10)
	if err != nil {
		t.Fatalf("SearchDuckDuckGo failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result")
	}
	t.Logf("Got %d search results", len(results))
	for i, r := range results {
		t.Logf("  %d. %s", i+1, r)
	}
}

func TestFetchAndSanitize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	text, hash, err := FetchAndSanitize(ctx, "https://example.com", 10)
	if err != nil {
		t.Fatalf("FetchAndSanitize failed: %v", err)
	}
	if len(text) == 0 {
		t.Fatal("expected non-empty text")
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	t.Logf("Got %d chars, hash=%s", len(text), hash[:16])
}

func TestExtractLinks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	links, err := ExtractLinks(ctx, "https://example.com", "", 10)
	if err != nil {
		t.Fatalf("ExtractLinks failed: %v", err)
	}
	t.Logf("Found %d links", len(links))
	for i, l := range links {
		t.Logf("  %d. %s", i+1, l)
	}
}

func TestGenerateContentHash(t *testing.T) {
	hash1 := GenerateContentHash("Hello World")
	hash2 := GenerateContentHash("Hello World ")
	hash3 := GenerateContentHash("Different content")

	if hash1 != hash2 {
		t.Error("hash should be same for whitespace-collapsed same content")
	}
	if hash1 == hash3 {
		t.Error("hash should differ for different content")
	}
	t.Logf("Hash: %s", hash1[:16])
}
