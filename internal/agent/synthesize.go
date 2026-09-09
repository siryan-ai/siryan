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

	blocks, claims, notes, err := synthesizeWithModel(query, sources)
	if err == nil && len(blocks) > 0 {
		return blocks, claims, notes, nil
	}

	note := "source_summary"
	if err != nil {
		note = "source_summary | " + err.Error()
	}
	return synthesizeFromSources(query, sources), nil, note, nil
}

func synthesizeFromSources(query string, sources []Source) []map[string]interface{} {
	var b strings.Builder
	b.WriteString("**" + query + "**\n\n")
	for _, s := range sources {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			text = strings.TrimSpace(s.Snippet)
		}
		if text == "" {
			continue
		}
		if len(text) > 320 {
			text = text[:320] + "…"
		}
		title := s.Title
		if title == "" {
			title = DomainOf(s.URL)
		}
		b.WriteString("### " + title + "\n")
		b.WriteString(text + "\n\n")
		b.WriteString("Kaynak: " + s.URL + "\n\n")
	}

	items := make([]map[string]interface{}, 0, len(sources))
	for _, s := range sources {
		desc := s.Snippet
		if desc == "" {
			desc = s.Text
		}
		items = append(items, map[string]interface{}{
			"title":       nonEmpty(s.Title, DomainOf(s.URL)),
			"subtitle":    DomainOf(s.URL),
			"description": trimSnippet(desc, 160),
			"url":         s.URL,
		})
	}

	return []map[string]interface{}{
		{
			"type":    "text",
			"version": 1,
			"data":    map[string]interface{}{"markdown": strings.TrimSpace(b.String())},
		},
		{
			"type":    "cards",
			"version": 1,
			"data":    map[string]interface{}{"items": items},
		},
	}
}

func synthesizeWithModel(query string, sources []Source) ([]map[string]interface{}, []Claim, string, error) {
	var b strings.Builder
	b.WriteString("Sorgu: " + query + "\n\n")
	b.WriteString("Sadece verilen kaynaklara dayan. Uydurma. Çelişki varsa yaz.\n\n")
	for i, s := range sources {
		body := s.Text
		if body == "" {
			body = s.Snippet
		}
		b.WriteString(fmt.Sprintf("--- KAYNAK %d ---\nURL: %s\nBaşlık: %s\nMetin: %s\n\n",
			i+1, s.URL, s.Title, truncate(body, 3500)))
	}
	b.WriteString(`SADECE şu JSON:
{
  "summary": "markdown kısa özet",
  "claims": [
    { "text": "...", "support_urls": ["https://..."], "conflict_urls": [], "confidence": 0.0 }
  ],
  "notes": "doğrulama notu"
}
`)

	raw, err := callGroqJSON(b.String())
	if err != nil {
		return nil, nil, "", err
	}

	clean := strings.TrimSpace(raw)
	if i := strings.Index(clean, "{"); i >= 0 {
		if j := strings.LastIndex(clean, "}"); j > i {
			clean = clean[i : j+1]
		}
	}

	var parsed struct {
		Summary string  `json:"summary"`
		Claims  []Claim `json:"claims"`
		Notes   string  `json:"notes"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, nil, "", fmt.Errorf("parse: %w", err)
	}
	if strings.TrimSpace(parsed.Summary) == "" {
		return nil, nil, "", fmt.Errorf("empty summary")
	}

	blocks := []map[string]interface{}{
		{
			"type":    "text",
			"version": 1,
			"data":    map[string]interface{}{"markdown": parsed.Summary},
		},
	}

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

	items := make([]map[string]interface{}, 0, len(sources))
	for _, s := range sources {
		items = append(items, map[string]interface{}{
			"title":       nonEmpty(s.Title, DomainOf(s.URL)),
			"subtitle":    DomainOf(s.URL),
			"description": trimSnippet(nonEmpty(s.Snippet, s.Text), 160),
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

func callGroqJSON(userContent string) (string, error) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return "", fmt.Errorf("no groq key")
	}
	payload := map[string]interface{}{
		"model": "qwen/qwen3.6-27b",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "Return ONLY valid JSON. No markdown fences. No <think> tags. No extra text.",
			},
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
		return "", fmt.Errorf("groq %d: %s", resp.StatusCode, truncate(string(raw), 180))
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
	return strings.TrimSpace(gr.Choices[0].Message.Content), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func trimSnippet(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
