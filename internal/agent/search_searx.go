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

func searchSearx(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SEARXNG_URL")), "/")
	if base == "" {
		return nil, fmt.Errorf("SEARXNG_URL empty")
	}
	if limit <= 0 {
		limit = 5
	}
	u := fmt.Sprintf("%s/search?q=%s&format=json&categories=general",
		base, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SiryanAgent/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("searx %d", resp.StatusCode)
	}

	var parsed struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, limit)
	for _, r := range parsed.Results {
		if r.URL == "" {
			continue
		}
		hits = append(hits, SearchHit{URL: r.URL, Title: r.Title, Snippet: r.Content})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}
