package partners

import "github.com/siryan-ai/siryan/internal/agent"

// TryPartners: anlaşmalı API'ler. Şimdilik hepsi false.
func TryPartners(userMessage string) (blocks []map[string]interface{}, sources []agent.Source, ok bool) {
	// örnek: trendyol adapter
	if blocks, sources, ok = TryTrendyol(userMessage); ok {
		return blocks, sources, true
	}
	return nil, nil, false
}
