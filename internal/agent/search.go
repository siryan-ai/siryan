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

	hits, err := searchDDG(parent, query, limit)
	if err == nil && len(hits) > 0 {
		return hits, nil
	}

	hits2, err2 := searchBing(parent, query, limit)
	if err2 == nil && len(hits2) > 0 {
		return hits2, nil
	}

	if err != nil {
		return nil, err
	}
	if err2 != nil {
		return nil, err2
	}
	return []SearchHit{}, nil
}

func searchDDG(parent context.Context, query string, limit int) ([]SearchHit, error) {
	ctx, cancel := context.WithTimeout(parent, 40*time.Second)
	defer cancel()

	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	allocCtx, allocCancel := chromeAllocator(ctx)
	defer allocCancel()
	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var items []map[string]string
	js := `(() => {
	  const out = [];
	  const push = (title, href, snippet) => {
	    if (!title || !href) return;
	    try {
	      const u = new URL(href, location.origin);
	      href = u.href;
	      const uddg = u.searchParams.get('uddg');
	      if (uddg) href = decodeURIComponent(uddg);
	    } catch (e) {}
	    if (!href.startsWith('http')) return;
	    if (href.includes('duckduckgo.com')) return;
	    out.push({ title: title.trim(), url: href, snippet: (snippet||'').trim() });
	  };
	  document.querySelectorAll('a.result__a').forEach(a => {
	    let sn = '';
	    const p = a.closest('.result');
	    if (p) {
	      const s = p.querySelector('.result__snippet, a.result__snippet');
	      if (s) sn = s.textContent || '';
	    }
	    push(a.textContent, a.href, sn);
	  });
	  document.querySelectorAll('a[data-testid="result-title-a"], a.result-link').forEach(a => {
	    push(a.textContent, a.href, '');
	  });
	  return out;
	})()`

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(js, &items),
	)
	if err != nil {
		return nil, err
	}
	return dedupeHits(items, limit), nil
}

func searchBing(parent context.Context, query string, limit int) ([]SearchHit, error) {
	ctx, cancel := context.WithTimeout(parent, 40*time.Second)
	defer cancel()

	searchURL := "https://www.bing.com/search?q=" + url.QueryEscape(query)
	allocCtx, allocCancel := chromeAllocator(ctx)
	defer allocCancel()
	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var items []map[string]string
	js := `(() => {
	  const out = [];
	  document.querySelectorAll('#b_results > li.b_algo').forEach(li => {
	    const a = li.querySelector('h2 a');
	    if (!a) return;
	    const title = (a.textContent || '').trim();
	    const href = a.href || '';
	    let snippet = '';
	    const c = li.querySelector('.b_caption p, .b_lineclamp2, .b_lineclamp3');
	    if (c) snippet = (c.textContent || '').trim();
	    if (title && href && href.startsWith('http') && !href.includes('bing.com')) {
	      out.push({ title, url: href, snippet });
	    }
	  });
	  return out;
	})()`

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(js, &items),
	)
	if err != nil {
		return nil, err
	}
	return dedupeHits(items, limit), nil
}

func dedupeHits(items []map[string]string, limit int) []SearchHit {
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
	return hits
}
