package agent

import (
	"context"
	"strings"
)

// SearchMulti: plan sorgularını çalıştır, birleştir. Hata UI'ye gitmez.
func SearchMulti(ctx context.Context, queries []string, perQuery int) []SearchHit {
	var all []SearchHit
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		hits := searchOneQuery(ctx, q, perQuery)
		all = append(all, hits...)
	}
	return dedupeHits(all, 12)
}

func searchOneQuery(ctx context.Context, query string, limit int) []SearchHit {
	// 1 SearX
	if hits, err := searchSearx(ctx, query, limit); err == nil && len(hits) > 0 {
		return hits
	}
	// 2 DDG HTML / Bing / Instant — mevcut fonksiyonların varsa çağır
	if hits, err := searchDDGHTML(ctx, query, limit); err == nil && len(hits) > 0 {
		return hits
	}
	if hits, err := searchBingHTML(ctx, query, limit); err == nil && len(hits) > 0 {
		return hits
	}
	if hits, err := searchDDGInstant(ctx, query, limit); err == nil && len(hits) > 0 {
		return hits
	}
	// 3 wiki yedek
	if hits, err := searchWikipedia(ctx, "tr", query, 2); err == nil && len(hits) > 0 {
		return hits
	}
	if hits, err := searchWikipedia(ctx, "en", query, 2); err == nil {
		return hits
	}
	return nil
}
