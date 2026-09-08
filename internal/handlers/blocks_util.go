// internal/handlers/blocks_util.go

package handlers

func FirstTextFromBlocks(blocks []map[string]interface{}) string {
	for _, b := range blocks {
		if b["type"] != "text" {
			continue
		}
		data, ok := b["data"].(map[string]interface{})
		if !ok {
			continue
		}
		if md, ok := data["markdown"].(string); ok && md != "" {
			return md
		}
	}
	return "Araştırma tamamlandı"
}
