package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

func Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if req.MaxURLs <= 0 {
		req.MaxURLs = 4
	}
	if req.Locale == "" {
		req.Locale = "tr"
	}

	steps := []string{"İstek analiz ediliyor"}

	if blocks, srcs, ok := tryPartnersLocal(req.UserMessage); ok {
		return &RunResult{
			UsedAgent: true,
			Blocks:    blocks,
			Sources:   srcs,
			Content:   firstMarkdown(blocks),
			Notes:     "partner",
			Steps:     []string{"Partner verisi"},
		}, nil
	}

	steps = append(steps, "Arama planı")
	plan := PlanQueries(req.UserMessage)

	steps = append(steps, fmt.Sprintf("%d sorgu", len(plan.Queries)))
	hits := SearchMulti(ctx, plan.Queries, 4)

	steps = append(steps, "Sayfalar okunuyor")
	sources := fetchAll(ctx, hits, plan, req.MaxURLs)
	sources = FilterRelevant(req.UserMessage, sources)

	steps = append(steps, "Derleniyor")
	blocks, claims, notes, _ := SynthesizeBlocks(req.UserMessage, sources)
	if len(blocks) == 0 {
		blocks = []map[string]interface{}{
			{
				"type": "text", "version": 1,
				"data": map[string]interface{}{
					"markdown": "Şu an yeterli açık kaynak derleyemedim. Soruyu netleştirip tekrar dene.",
				},
			},
		}
	}
	blocks = appendSourceList(blocks, sources)

	return &RunResult{
		UsedAgent: true,
		Blocks:    blocks,
		Sources:   sources,
		Claims:    claims,
		Notes:     notes,
		Content:   firstMarkdown(blocks),
		Steps:     steps,
	}, nil
}

func tryPartnersLocal(userMessage string) ([]map[string]interface{}, []Source, bool) {
	_ = userMessage
	return nil, nil, false
}

func fetchAll(ctx context.Context, hits []SearchHit, plan SearchPlan, max int) []Source {
	if len(hits) == 0 {
		return nil
	}
	if max > len(hits) {
		max = len(hits)
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []Source
		sem = make(chan struct{}, 2)
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
	n := 0
	for _, s := range sources {
		if strings.Contains(strings.ToLower(s.URL), "wikipedia.org") {
			continue
		}
		t := s.Title
		if t == "" {
			t = DomainOf(s.URL)
		}
		b.WriteString("- [" + t + "](" + s.URL + ")\n")
		n++
	}
	if n == 0 {
		return blocks
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

func RunResearch(ctx context.Context, req ResearchRequest) (*ResearchResult, error) {
	res, err := Run(ctx, RunRequest{
		UserMessage: req.Query,
		Locale:      req.Locale,
		MaxURLs:     req.MaxURLs,
	})
	if err != nil || res == nil {
		return &ResearchResult{
			Query: req.Query,
			Blocks: []map[string]interface{}{
				{"type": "text", "version": 1, "data": map[string]interface{}{
					"markdown": "Şu an net sonuç derleyemedim.",
				}},
			},
			Notes: "silent_fail",
		}, nil
	}
	return &ResearchResult{
		Query:   req.Query,
		Sources: res.Sources,
		Claims:  res.Claims,
		Blocks:  res.Blocks,
		Notes:   res.Notes,
	}, nil
}
