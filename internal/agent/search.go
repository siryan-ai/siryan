// internal/agent/search.go
// Öncelik: geniş web (Brave → DDG HTML → Bing HTML → DDG Instant) ; Wikipedia sadece ek kaynak

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

	// 1) Brave — gerçek web araması (key varsa)
	if key := strings.TrimSpace(os.Getenv("BRAVE_API_KEY")); key != "" {
		hits, err := searchBrave(ctx, query, limit, key)
		if err != nil {
			errs = append(errs, "brave:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// 2) DuckDuckGo HTML — genel web
	if len(all) < limit {
		hits, err := searchDDGHTML(ctx, query, limit*2)
		if err != nil {
			errs = append(errs, "ddg_html:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// 3) Bing HTML — genel web
	if len(all) < limit {
		hits, err := searchBingHTML(ctx, query, limit*2)
		if err != nil {
			errs = append(errs, "bing:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// 4) DDG Instant
	if len(all) < limit {
		hits, err := searchDDGInstant(ctx, query, limit)
		if err != nil {
			errs = append(errs, "ddg_api:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// 5) Wikipedia — sadece yedek / ek (tek başına ana kaynak değil)
	if len(all) < 2 {
		if hits, err := searchWikipedia(ctx, "en", query, 3); err != nil {
			errs = append(errs, "wiki_en:"+err.Error())
		} else {
			all = append(all, hits...)
		}
		if hits, err := searchWikipedia(ctx, "tr", query, 2); err != nil {
			errs = append(errs, "wiki_tr:"+err.Error())
		} else {
			all = append(all, hits...)
		}
	}

	// Wiki ağırlığını kır: mümkünse non-wiki önce
	out := preferNonWiki(dedupeHits(all, limit*2), limit)
	if len(out) == 0 {
		msg := "all_search_backends_empty"
		if len(errs) > 0 {
			msg += " | " + strings.Join(errs, " ; ")
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return out, nil
}

func preferNonWiki(hits []SearchHit, limit int) []SearchHit {
	var web, wiki []SearchHit
	for _, h := range hits {
		if strings.Contains(strings.ToLower(h.URL), "wikipedia.org") {
			wiki = append(wiki, h)
		} else {
			web = append(web, h)
		}
	}
	out := append([]SearchHit{}, web...)
	if len(out) < limit {
		out = append(out, wiki...)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

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

	re := regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(html, limit*4)
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

func searchBingHTML(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	u := "https://www.bing.com/search?q=" + url.QueryEscape(query) + "&setlang=en-us"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

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

	// Bing algo sonuçları
	re := regexp.MustCompile(`(?s)<li class="b_algo".*?<h2[^>]*>\s*<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(html, limit*4)
	var hits []SearchHit
	seen := map[string]bool{}
	for _, m := range matches {
		href := htmlUnescape(m[1])
		title := stripTags(m[2])
		if !strings.HasPrefix(href, "http") || seen[href] || strings.Contains(href, "bing.com") || strings.Contains(href, "microsoft.com") {
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
