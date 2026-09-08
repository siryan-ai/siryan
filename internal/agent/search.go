// internal/agent/search.go

package agent

import (
	"context"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type SearchHit struct {
	URL     string
	Title   string
	Snippet string
}

// DuckDuckGo HTML (API key yok). Kırılgan olabilir; SearXNG ile değiştirilebilir.
func SearchWeb(parent context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 5
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	q := url.QueryEscape(query)
	searchURL := "https://html.duckduckgo.com/html/?q=" + q

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(os.Getenv("CHROME_PATH")), // boşsa default
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"),
		)...,
	)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var items []map[string]string
	js := `(() => {
	  const out = [];
	  const nodes = document.querySelectorAll('a.result__a');
	  nodes.forEach((a) => {
	    const title = (a.textContent || '').trim();
	    let href = a.href || '';
	    // DDG redirect
	    try {
	      const u = new URL(href);
	      const uddg = u.searchParams.get('uddg');
	      if (uddg) href = decodeURIComponent(uddg);
	    } catch (e) {}
	    let snippet = '';
	    const parent = a.closest('.result');
	    if (parent) {
	      const sn = parent.querySelector('.result__snippet');
	      if (sn) snippet = (sn.textContent || '').trim();
	    }
	    if (title && href && href.startsWith('http')) {
	      out.push({ title, url: href, snippet });
	    }
	  });
	  return out;
	})()`

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1200*time.Millisecond),
		chromedp.Evaluate(js, &items),
	)
	if err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, limit)
	seen := map[string]bool{}
	for _, it := range items {
		u := strings.TrimSpace(it["url"])
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		hits = append(hits, SearchHit{
			URL:     u,
			Title:   strings.TrimSpace(it["title"]),
			Snippet: strings.TrimSpace(it["snippet"]),
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}
