package agent

import (
	"context"
	"strings"
)

func SearchMulti(ctx context.Context, queries []string, perQuery int) []SearchHit {
	if perQuery <= 0 {
		perQuery = 4
	}
	var all []SearchHit
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		hits := searchOneQuery(ctx, q, perQuery)
		all = append(all, hits...)
	}
	return dedupeHits(all, 8)
}

func searchOneQuery(ctx context.Context, query string, limit int) []SearchHit {
	if hits, err := searchSearx(ctx, query, limit); err == nil && len(hits) > 0 {
		return filterOutWiki(hits)
	}
	return nil
}

func filterOutWiki(hits []SearchHit) []SearchHit {
	out := make([]SearchHit, 0, len(hits))
	for _, h := range hits {
		u := strings.ToLower(h.URL)
		if strings.Contains(u, "wikipedia.org") || strings.Contains(u, "wikimedia.org") {
			continue
		}
		out = append(out, h)
	}
	return out
}

func dedupeHits(items []SearchHit, limit int) []SearchHit {
	out := make([]SearchHit, 0, limit)
	seen := map[string]bool{}
	for _, h := range items {
		u := strings.TrimSpace(h.URL)
		if u == "" || seen[u] {
			continue
		}
		if strings.Contains(strings.ToLower(u), "wikipedia.org") {
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
