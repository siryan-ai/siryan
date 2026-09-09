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

var searchHTTP = &http.Client{Timeout: 30 * time.Second}

func SearchWeb(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 5
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	var all []SearchHit
	var errs []string

	// 1) Kendi SearXNG (ana yol)
	if base := strings.TrimRight(strings.TrimSpace(os.Getenv("SEARXNG_URL")), "/"); base != "" {
		hits, err := searchSearXNG(ctx, base, query, limit)
		if err != nil {
			errs = append(errs, "searx:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// 2) Yedekler (SearX boşsa)
	if len(all) < limit {
		if hits, err := searchDDGInstant(ctx, query, limit); err != nil {
			errs = append(errs, "ddg:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}
	if len(all) < 2 {
		if hits, err := searchWikipedia(ctx, "en", query, 3); err != nil {
			errs = append(errs, "wiki:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	out := dedupeHits(all, limit)
	if len(out) == 0 {
		msg := "search_empty"
		if len(errs) > 0 {
			msg += " | " + strings.Join(errs, " ; ")
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return out, nil
}

func searchSearXNG(ctx context.Context, base, query string, limit int) ([]SearchHit, error) {
	u := fmt.Sprintf("%s/search?q=%s&format=json&language=all",
		base, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SiryanResearch/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := searchHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, trim(string(body), 150))
	}

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	hits := make([]SearchHit, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.URL == "" {
			continue
		}
		hits = append(hits, SearchHit{
			URL:     r.URL,
			Title:   r.Title,
			Snippet: r.Content,
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func searchWikipedia(ctx context.Context, lang, query string, limit int) ([]SearchHit, error) {
	api := fmt.Sprintf(
		"https://%s.wikipedia.org/w/api.php?action=opensearch&limit=%d&namespace=0&format=json&search=%s",
		lang, limit, url.QueryEscape(query),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SiryanResearch/1.0 (siryan-ai)")
	resp, err := searchHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil || len(arr) < 4 {
		return nil, fmt.Errorf("bad opensearch")
	}
	var titles, descs, urls []string
	_ = json.Unmarshal(arr[1], &titles)
	_ = json.Unmarshal(arr[2], &descs)
	_ = json.Unmarshal(arr[3], &urls)
	hits := make([]SearchHit, 0, len(titles))
	for i := range titles {
		if i >= len(urls) || urls[i] == "" {
			continue
		}
		sn := ""
		if i < len(descs) {
			sn = descs[i]
		}
		hits = append(hits, SearchHit{Title: titles[i], URL: urls[i], Snippet: sn})
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
	resp, err := searchHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var parsed struct {
		AbstractURL  string `json:"AbstractURL"`
		AbstractText string `json:"AbstractText"`
		Heading      string `json:"Heading"`
		Results      []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	var hits []SearchHit
	if parsed.AbstractURL != "" {
		t := parsed.Heading
		if t == "" {
			t = parsed.AbstractURL
		}
		hits = append(hits, SearchHit{URL: parsed.AbstractURL, Title: t, Snippet: parsed.AbstractText})
	}
	for _, r := range parsed.Results {
		if r.FirstURL != "" {
			hits = append(hits, SearchHit{URL: r.FirstURL, Title: r.Text, Snippet: r.Text})
		}
	}
	for _, r := range parsed.RelatedTopics {
		if r.FirstURL != "" {
			hits = append(hits, SearchHit{URL: r.FirstURL, Title: r.Text, Snippet: r.Text})
		}
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func dedupeHits(items []SearchHit, limit int) []SearchHit {
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

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
