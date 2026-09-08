// internal/agent/synthesize.go

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func SynthesizeBlocks(query string, sources []Source) ([]map[string]interface{}, []Claim, string, error) {
	if len(sources) == 0 {
		blocks := []map[string]interface{}{
			{
				"type":    "text",
				"version": 1,
				"data": map[string]interface{}{
					"markdown": "Kaynak bulunamadı. Sorguyu daraltıp tekrar dene.",
				},
			},
		}
		return blocks, nil, "no_sources", nil
	}

	var b strings.Builder
	b.WriteString("Sorgu: " + query + "\n\n")
	b.WriteString("Aşağıdaki kaynaklara DAYALI özet çıkar. Kaynakta yoksa uydurma.\n")
	b.WriteString("Çelişki varsa açık yaz. Güncel değilse belirt.\n\n")
	for i, s := range sources {
		b.WriteString(fmt.Sprintf("--- KAYNAK %d ---\nURL: %s\nBaşlık: %s\nMetin: %s\n\n",
			i+1, s.URL, s.Title, truncate(s.Text, 3500)))
	}
	b.WriteString(`SADECE şu JSON'u döndür:
{
  "summary": "kısa markdown özet",
  "claims": [
    { "text": "...", "support_urls": ["https://..."], "conflict_urls": [], "confidence": 0.0 }
  ],
  "notes": "doğrulama notu"
}
confidence 0-1. support_urls sadece verilen kaynaklardan.
`)

	raw, err := callGroqJSON(b.String())
	if err != nil {
		// model yoksa kaba özet
		return fallbackBlocks(query, sources), nil, "model_fallback", nil
	}

	var parsed struct {
		Summary string  `json:"summary"`
		Claims  []Claim `json:"claims"`
		Notes   string  `json:"notes"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fallbackBlocks(query, sources), nil, "parse_fallback", nil
	}

	blocks := []map[string]interface{}{}
	if parsed.Summary != "" {
		blocks = append(blocks, map[string]interface{}{
			"type":    "text",
			"version": 1,
			"data":    map[string]interface{}{"markdown": parsed.Summary},
		})
	}

	// claims table
	if len(parsed.Claims) > 0 {
		cols := []string{"İddia", "Güven", "Kaynaklar"}
		rows := [][]string{}
		for _, c := range parsed.Claims {
			rows = append(rows, []string{
				c.Text,
				fmt.Sprintf("%.0f%%", c.Confidence*100),
				strings.Join(c.Support, " "),
			})
		}
		blocks = append(blocks, map[string]interface{}{
			"type":    "table",
			"version": 1,
			"data": map[string]interface{}{
				"caption": "Doğrulama",
				"columns": cols,
				"rows":    rows,
			},
		})
	}

	// source cards
	items := []map[string]interface{}{}
	for _, s := range sources {
		items = append(items, map[string]interface{}{
			"title":       s.Title,
			"subtitle":    DomainOf(s.URL),
			"description": s.Snippet,
			"url":         s.URL,
		})
	}
	blocks = append(blocks, map[string]interface{}{
		"type":    "cards",
		"version": 1,
		"data":    map[string]interface{}{"items": items},
	})

	if parsed.Notes != "" {
		blocks = append(blocks, map[string]interface{}{
			"type":    "text",
			"version": 1,
			"data":    map[string]interface{}{"markdown": "_" + parsed.Notes + "_"},
		})
	}

	return blocks, parsed.Claims, parsed.Notes, nil
}

func fallbackBlocks(query string, sources []Source) []map[string]interface{} {
	var lines []string
	lines = append(lines, "**Araştırma:** "+query)
	for _, s := range sources {
		lines = append(lines, fmt.Sprintf("- [%s](%s): %s", s.Title, s.URL, s.Snippet))
	}
	items := []map[string]interface{}{}
	for _, s := range sources {
		items = append(items, map[string]interface{}{
			"title": s.Title, "url": s.URL, "description": s.Snippet,
		})
	}
	return []map[string]interface{}{
		{"type": "text", "version": 1, "data": map[string]interface{}{"markdown": strings.Join(lines, "\n")}},
		{"type": "cards", "version": 1, "data": map[string]interface{}{"items": items}},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func callGroqJSON(userContent string) (string, error) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return "", fmt.Errorf("no groq key")
	}
	payload := map[string]interface{}{
		"model": "qwen/qwen3.6-27b",
		"messages": []map[string]string{
			{"role": "system", "content": "Kaynaklara bağlı kal. JSON dışında bir şey yazma."},
			{"role": "user", "content": userContent},
		},
		"temperature": 0.2,
		"max_tokens":  2048,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("groq %d", resp.StatusCode)
	}
	var gr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &gr); err != nil || len(gr.Choices) == 0 {
		return "", fmt.Errorf("bad groq body")
	}
	content := strings.TrimSpace(gr.Choices[0].Message.Content)
	// json extract
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	return content, nil
}
