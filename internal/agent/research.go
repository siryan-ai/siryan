// internal/agent/research.go

package agent

import (
	"context"
	"sync"
)

func RunResearch(ctx context.Context, req ResearchRequest) (*ResearchResult, error) {
	if req.MaxURLs <= 0 {
		req.MaxURLs = 5
	}
	if req.Locale == "" {
		req.Locale = "tr"
	}

	hits, err := SearchWeb(ctx, req.Query, req.MaxURLs)
	if err != nil {
		return &ResearchResult{
			Query: req.Query,
			Notes: "search_failed: " + err.Error(),
			Blocks: []map[string]interface{}{
				{"type": "text", "version": 1, "data": map[string]interface{}{
					"markdown": "Arama yapılamadı. " + err.Error(),
				}},
			},
		}, nil
	}

	sources := make([]Source, 0, len(hits))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// paralel fetch (max 3 aynı anda)
	sem := make(chan struct{}, 3)
	for _, h := range hits {
		h := h
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			title, text, err := FetchPage(ctx, h.URL)
			if err != nil {
				mu.Lock()
				sources = append(sources, Source{
					URL: h.URL, Title: h.Title, Snippet: h.Snippet, Text: "", Score: 0.2,
				})
				mu.Unlock()
				return
			}
			src := BuildSource(h, title, text)
			mu.Lock()
			sources = append(sources, src)
			mu.Unlock()
		}()
	}
	wg.Wait()

	domainN := UniqueDomains(sources)
	base := ScoreByDomains(domainN)
	for i := range sources {
		if sources[i].Text != "" {
			sources[i].Score = base
		}
	}

	blocks, claims, notes, _ := SynthesizeBlocks(req.Query, sources)

	return &ResearchResult{
		Query:   req.Query,
		Sources: sources,
		Claims:  claims,
		Blocks:  blocks,
		Notes:   notes,
	}, nil
}
