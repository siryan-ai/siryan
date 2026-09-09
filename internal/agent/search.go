// internal/agent/search.go

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type SearchHit struct {
	URL     string
	Title   string
	Snippet string
}

var searchHTTP = &http.Client{
	Timeout: 25 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

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

	// 1) Brave (opsiyonel)
	if key := strings.TrimSpace(os.Getenv("BRAVE_API_KEY")); key != "" {
		hits, err := searchBrave(ctx, query, limit, key)
		if err != nil {
			errs = append(errs, "brave:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// 2) Wikipedia EN
	if hits, err := searchWikipedia(ctx, "en", query, limit); err != nil {
		errs = append(errs, "wiki_en:"+err.Error())
	} else {
		all = append(all, hits...)
	}

	// 3) Wikipedia TR
	if hits, err := searchWikipedia(ctx, "tr", query, limit); err != nil {
		errs = append(errs, "wiki_tr:"+err.Error())
	} else {
		all = append(all, hits...)
	}

	// 4) DDG Instant Answer API
	if hits, err := searchDDGInstant(ctx, query, limit); err != nil {
		errs = append(errs, "ddg_api:"+err.Error())
	} else {
		all = append(all, hits...)
	}

	// 5) DDG HTML (http get, chrome yok)
	if len(all) < limit {
		if hits, err := searchDDGHTML(ctx, query, limit); err != nil {
			errs = append(errs, "ddg_html:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	out := dedupeHits(all, limit)
	if len(out) == 0 {
		// Son çare: wiki arama sayfası + doğrudan wiki title guess
		out = fallbackWikiURLs(query)
	}
	if len(out) == 0 {
		msg := "all_search_backends_empty"
		if len(errs) > 0 {
			msg += " | " + strings.Join(errs, " ; ")
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return out, nil
}

func searchBrave(ctx context.Context, query string, limit int, key string) ([]SearchHit, error) {
	u := fmt.Sprintf(
		"https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), limit,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", key)
	req.Header.Set("User-Agent", "SiryanResearch/1.0")

	resp, err := searchHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncateErr(body))
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
		if r.URL == "" {
			continue
		}
		hits = append(hits, SearchHit{URL: r.URL, Title: r.Title, Snippet: r.Description})
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
	req.Header.Set("User-Agent", "SiryanResearch/1.0 (siryan-ai; research-agent)")
	req.Header.Set("Accept", "application/json")

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
		return nil, fmt.Errorf("bad opensearch body")
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
	req.Header.Set("Accept", "application/json")

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
		title := parsed.Heading
		if title == "" {
			title = parsed.AbstractURL
		}
		hits = append(hits, SearchHit{
			URL: parsed.AbstractURL, Title: title, Snippet: parsed.AbstractText,
		})
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

func searchDDGHTML(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	u := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SiryanResearch/1.0)")
	req.Header.Set("Accept", "text/html")

	resp, err := searchHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	html := string(body)

	// result__a href + text
	re := regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(html, limit*3)
	var hits []SearchHit
	seen := map[string]bool{}
	for _, m := range matches {
		href := htmlUnescape(m[1])
		title := stripTags(m[2])
		if strings.Contains(href, "uddg=") {
			if uq, err := url.Parse(href); err == nil {
				if real := uq.Query().Get("uddg"); real != "" {
					href, _ = url.QueryUnescape(real)
				}
			}
		}
		if !strings.HasPrefix(href, "http") || seen[href] || strings.Contains(href, "duckduckgo.com") {
			continue
		}
		seen[href] = true
		hits = append(hits, SearchHit{URL: href, Title: title, Snippet: ""})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func fallbackWikiURLs(query string) []SearchHit {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	title := strings.ReplaceAll(q, " ", "_")
	return []SearchHit{
		{
			URL:     "https://en.wikipedia.org/wiki/" + url.PathEscape(title),
			Title:   q + " (Wikipedia EN guess)",
			Snippet: "Fallback direct wiki path",
		},
		{
			URL:     "https://tr.wikipedia.org/wiki/" + url.PathEscape(title),
			Title:   q + " (Wikipedia TR guess)",
			Snippet: "Fallback direct wiki path",
		},
	}
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

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	s = re.ReplaceAllString(s, "")
	return strings.TrimSpace(htmlUnescape(s))
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
	)
	return r.Replace(s)
}

func truncateErr(b []byte) string {
	s := string(b)
	if len(s) > 120 {
		return s[:120]
	}
	return s
}
