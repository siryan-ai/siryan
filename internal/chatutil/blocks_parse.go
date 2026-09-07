package chatutil

import (
	"encoding/json"
	"regexp"
	"strings"
)

type Block struct {
	Type    string                 `json:"type"`
	Version int                    `json:"version"`
	Data    map[string]interface{} `json:"data"`
}

type BlocksPayload struct {
	Blocks []Block `json:"blocks"`
}

var fencedJSON = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func ParseBlocks(raw string) (blocks []Block, content string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", false
	}

	// think temizliği önce
	clean, _ := SplitThinking(raw)
	if clean == "" {
		clean = raw
	}
	clean = strings.TrimSpace(clean)

	candidates := []string{clean}
	if m := fencedJSON.FindStringSubmatch(clean); len(m) == 2 {
		candidates = append([]string{strings.TrimSpace(m[1])}, candidates...)
	}
	// metin içinde ilk { ... son }
	if i := strings.Index(clean, "{"); i >= 0 {
		if j := strings.LastIndex(clean, "}"); j > i {
			candidates = append([]string{clean[i : j+1]}, candidates...)
		}
	}

	for _, c := range candidates {
		var p BlocksPayload
		if err := json.Unmarshal([]byte(c), &p); err != nil {
			continue
		}
		if len(p.Blocks) == 0 {
			continue
		}
		// normalize version
		for i := range p.Blocks {
			if p.Blocks[i].Version == 0 {
				p.Blocks[i].Version = 1
			}
			if p.Blocks[i].Data == nil {
				p.Blocks[i].Data = map[string]interface{}{}
			}
		}
		summary := blocksSummary(p.Blocks)
		return p.Blocks, summary, true
	}

	// JSON yok → tek text block
	return []Block{{
		Type:    "text",
		Version: 1,
		Data:    map[string]interface{}{"markdown": clean},
	}}, clean, true
}

func blocksSummary(blocks []Block) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			if md, ok := b.Data["markdown"].(string); ok && md != "" {
				parts = append(parts, md)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	if len(blocks) == 1 {
		return "[" + blocks[0].Type + "]"
	}
	return "[çoklu blok]"
}
