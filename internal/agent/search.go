package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type SearchHit struct {
	URL     string
	Title   string
	Snippet string
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func SearchWeb(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 5
	}

	// 1) Brave (opsiyonel key — en stabil)
	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		if hits, err := searchBrave(ctx, query, limit, key); err == nil && len(hits) > 0 {
			return hits, nil
		}
	}

	// 2) Wikipedia + DDG Instant Answer (keysiz)
	var all []SearchHit
	if hits, err := searchWikipedia(ctx, query, limit); err == nil {
		all = append(all, hits...)
	}
	if hits, err := searchDDGInstant(ctx, query, limit); err == nil {
		all = append(all, hits...)
	}

	return dedupeHitsMaps(all, limit), nil
}

func searchBrave(ctx context.Context, query string, limit int, key string) ([]SearchHit, error) {
	u := "https://api.search.brave.com/res/v1/web/search?q=" + url.QueryEscape(query) + "&count=" + fmt.Sprintf("%d", limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", key)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("brave %d", resp.StatusCode)
	}

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		hits = append(hits, SearchHit{URL: r.URL, Title: r.Title, Snippet: r.Description})
	}
	return hits, nil
}

func searchWikipedia(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	// opensearch
	api := "https://en.wikipedia.org/w/api.php?action=opensearch&limit=" + fmt.Sprintf("%d", limit) +
		"&namespace=0&format=json&search=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SiryanResearch/1.0 (contact: siryan-ai)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wiki opensearch %d", resp.StatusCode)
	}

	// ["query", [titles], [descs], [urls]]
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil || len(arr) < 4 {
		return nil, fmt.Errorf("wiki parse")
	}
	var titles, descs, urls []string
	_ = json.Unmarshal(arr[1], &titles)
	_ = json.Unmarshal(arr[2], &descs)
	_ = json.Unmarshal(arr[3], &urls)

	hits := make([]SearchHit, 0, len(titles))
	for i := range titles {
		u := ""
		if i < len(urls) {
			u = urls[i]
		}
		sn := ""
		if i < len(descs) {
			sn = descs[i]
		}
		if u == "" {
			continue
		}
		hits = append(hits, SearchHit{Title: titles[i], URL: u, Snippet: sn})
	}
	return hits, nil
}

func searchDDGInstant(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	u := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SiryanResearch/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ddg %d", resp.StatusCode)
	}

	var parsed struct {
		AbstractURL   string `json:"AbstractURL"`
		AbstractText  string `json:"AbstractText"`
		Heading       string `json:"Heading"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
		Results []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	var hits []SearchHit
	if parsed.AbstractURL != "" {
		hits = append(hits, SearchHit{
			URL:     parsed.AbstractURL,
			Title:   nonEmpty(parsed.Heading, parsed.AbstractURL),
			Snippet: parsed.AbstractText,
		})
	}
	for _, r := range parsed.Results {
		if r.FirstURL == "" {
			continue
		}
		hits = append(hits, SearchHit{URL: r.FirstURL, Title: r.Text, Snippet: r.Text})
	}
	for _, r := range parsed.RelatedTopics {
		if r.FirstURL == "" {
			continue
		}
		hits = append(hits, SearchHit{URL: r.FirstURL, Title: r.Text, Snippet: r.Text})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func dedupeHitsMaps(items []SearchHit, limit int) []SearchHit {
	out := make([]SearchHit, 0, limit)
	seen := map[string]bool{}
	for _, h := range items {
		u := strings.TrimSpace(h.URL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out
}
