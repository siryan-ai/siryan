package agent

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type SearchHit struct {
	URL     string
	Title   string
	Snippet string
}

func SearchWeb(parent context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 5
	}
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	defer cancel()

	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	allocCtx, allocCancel := chromeAllocator(ctx)
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
		chromedp.Sleep(1500*time.Millisecond),
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
