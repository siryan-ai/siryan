package agent

import (
	"context"
	"strings"
	"sync"
)

func Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if req.MaxURLs <= 0 {
		req.MaxURLs = 6
	}
	if req.Locale == "" {
		req.Locale = "tr"
	}

	// Layer C: partner (şimdilik stub — eşleşirse web'den önce)
	if blocks, srcs, ok := TryPartners(req.UserMessage); ok {
		return &RunResult{
			UsedAgent: true,
			Blocks:    blocks,
			Sources:   srcs,
			Content:   firstMarkdown(blocks),
			Notes:     "partner",
		}, nil
	}

	plan := PlanQueries(req.UserMessage)
	hits := SearchMulti(ctx, plan.Queries, 5)

	// Fetch paralel
	sources := fetchAll(ctx, hits, plan, req.MaxURLs)
	sources = FilterRelevant(req.UserMessage, sources)

	blocks, claims, notes, _ := SynthesizeBlocks(req.UserMessage, sources)
	// Synthesize asla boş bırakmasın — synthesizeFromSources garantisi

	// Kaynak listesi her zaman sonda
	blocks = appendSourceList(blocks, sources)

	return &RunResult{
		UsedAgent: true,
		Blocks:    blocks,
		Sources:   sources,
		Claims:    claims,
		Notes:     notes,
		Content:   firstMarkdown(blocks),
	}, nil
}

func fetchAll(ctx context.Context, hits []SearchHit, plan SearchPlan, max int) []Source {
	if max > len(hits) {
		max = len(hits)
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []Source
		sem = make(chan struct{}, 3)
	)
	for i := 0; i < max; i++ {
		h := hits[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			title, text, err := FetchPage(ctx, h.URL)
			src := BuildSource(h, title, text)
			if err != nil || strings.TrimSpace(src.Text) == "" {
				src.Text = h.Snippet
				src.Snippet = h.Snippet
				src.Score = 0.25
			}
			src.QueryUsed = strings.Join(plan.Queries, " | ")
			mu.Lock()
			out = append(out, src)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func appendSourceList(blocks []map[string]interface{}, sources []Source) []map[string]interface{} {
	if len(sources) == 0 {
		return blocks
	}
	var b strings.Builder
	b.WriteString("**Kaynaklar**\n")
	for i, s := range sources {
		t := s.Title
		if t == "" {
			t = s.URL
		}
		b.WriteString(strings.TrimSpace(strings.Join([]string{
			"", strings.Repeat("", 0),
		}, "")))
		b.WriteString(strings.Join([]string{
			string(rune('1' + i)), // basit; aşağıda düz index
		}, ""))
	}
	// düz liste
	b.Reset()
	b.WriteString("**Kaynaklar**\n")
	for i, s := range sources {
		t := s.Title
		if t == "" {
			t = DomainOf(s.URL)
		}
		b.WriteString(strings.TrimSpace(
			strings.Join([]string{"", ""}, ""),
		))
		b.WriteString(string(rune(0))) // no-op clean below
		_ = i
		b.WriteString("- [" + t + "](" + s.URL + ")\n")
	}
	return append(blocks, map[string]interface{}{
		"type": "text", "version": 1,
		"data": map[string]interface{}{"markdown": strings.TrimSpace(b.String())},
	})
}

func firstMarkdown(blocks []map[string]interface{}) string {
	for _, b := range blocks {
		if b["type"] == "text" {
			if d, ok := b["data"].(map[string]interface{}); ok {
				if md, ok := d["markdown"].(string); ok && md != "" {
					return md
				}
			}
		}
	}
	return "Tamam."
}
