package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

// FetchAndSanitize GETs a URL, preserves link URLs in text, and strips HTML.
func FetchAndSanitize(targetURL string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	return sanitizeHTML(resp.Body, targetURL)
}

func sanitizeHTML(body io.Reader, baseURLStr string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return "", err
	}

	// Remove unwanted elements
	doc.Find("script, style, nav, footer, header, noscript, iframe").Remove()

	base, _ := url.Parse(baseURLStr)

	// Preserve links by appending [Link: URL] to the link text
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

	// Extract text
	text := doc.Find("body").Text()

	// Clean up whitespace
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	return strings.Join(cleanLines, " "), nil
}

// ExtractLinks GETs a URL and returns absolute URLs that contain the specified pattern.
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

	var links []string
	seen := make(map[string]bool)

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			parsedHref, err := url.Parse(href)
			if err == nil {
				absURL := baseURL.ResolveReference(parsedHref).String()
				if strings.HasPrefix(absURL, "http") && strings.Contains(absURL, pattern) && !seen[absURL] {
					seen[absURL] = true
					links = append(links, absURL)
				}
			}
		}
	})

	return links, nil
}

// SearchDuckDuckGo performs a search on duckduckgo HTML and returns up to maxResults URLs.
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
