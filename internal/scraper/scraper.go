package scraper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func FetchAndSanitize(targetURL string) (string, string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	return sanitizeHTML(resp.Body, targetURL)
}

func sanitizeHTML(body io.Reader, baseURLStr string) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return "", "", err
	}

	doc.Find("script, style, nav, footer, header, noscript, iframe, aside").Remove()

	base, _ := url.Parse(baseURLStr)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && base != nil {
			parsedHref, err := url.Parse(href)
			if err == nil {
				absURL := base.ResolveReference(parsedHref).String()
				if strings.HasPrefix(absURL, "http") {
					s.SetText(fmt.Sprintf("%s [Link: %s]", strings.TrimSpace(s.Text()), absURL))
				}
			}
		}
	})

	text := doc.Find("body").Text()

	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	rawText := strings.Join(cleanLines, " ")
	hash := GenerateContentHash(rawText)

	return rawText, hash, nil
}

func GenerateContentHash(text string) string {
	lower := strings.ToLower(text)

	// 1. Relative Time (The Ticking Clocks)
	reRelativeTime := regexp.MustCompile(`(?i)\b\d+\s*(secs?|seconds?|mins?|minutes?|hrs?|hours?|days?|weeks?|months?|years?)\s*ago\b`)
	lower = reRelativeTime.ReplaceAllString(lower, "<TIME_AGO>")

	// 2. Engagement Metrics
	reMetrics := regexp.MustCompile(`(?i)\b\d+\s*(views?|comments?|likes?|replies|retweets|shares)\b`)
	lower = reMetrics.ReplaceAllString(lower, "<METRIC>")

	// 3. Parenthetical Counters (e.g., (45))
	reParenCount := regexp.MustCompile(`\(\s*\d+\s*\)`)
	lower = reParenCount.ReplaceAllString(lower, "<COUNT>")

	// 4. Dynamic Updates
	reUpdated := regexp.MustCompile(`(?i)(?:last\s*)?updated\s*(?:today|yesterday|now|\d+)`)
	lower = reUpdated.ReplaceAllString(lower, "<UPDATED>")

	// 5. Collapse Whitespace
	reWhitespace := regexp.MustCompile(`\s+`)
	final := reWhitespace.ReplaceAllString(lower, " ")

	hash := sha256.Sum256([]byte(strings.TrimSpace(final)))
	return hex.EncodeToString(hash[:])
}

func ExtractLinks(targetURL, pattern string) ([]string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Compile the regex pattern
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
	}

	var links []string
	seen := make(map[string]bool)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			parsedHref, err := url.Parse(href)
			if err == nil {
				absURL := baseURL.ResolveReference(parsedHref).String()
				if strings.HasPrefix(absURL, "http") && regex.MatchString(absURL) && !seen[absURL] {
					seen[absURL] = true
					links = append(links, absURL)
				}
			}
		}
	})

	return links, nil
}

func SearchDuckDuckGo(query string, maxResults int) ([]string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search status code error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []string
	doc.Find("a.result__url").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}
		href, exists := s.Attr("href")
		if exists {
			if strings.HasPrefix(href, "//duckduckgo.com/l/?uddg=") {
				u, _ := url.Parse(href)
				q := u.Query()
				if uddg := q.Get("uddg"); uddg != "" {
					href = uddg
				}
			}
			results = append(results, href)
		}
	})

	return results, nil
}
