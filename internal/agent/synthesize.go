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
					"markdown": "Yeterli açık kaynak bulamadım. Soruyu biraz daha netleştirip tekrar denerim.",
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
	// Önce birleşik kısa anlatım — sadece başlık+link listesi değil
	n := 0
	for _, s := range sources {
		t := strings.TrimSpace(s.Text)
		if t == "" {
			t = strings.TrimSpace(s.Snippet)
		}
		if t == "" {
			continue
		}
		if len(t) > 280 {
			t = t[:280] + "…"
		}
		if n == 0 {
			b.WriteString(t)
			b.WriteString("\n\n")
		} else {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		n++
		if n >= 4 {
			break
		}
	}
	if b.Len() == 0 {
		b.WriteString("Kaynaklara ulaşıldı ama okunabilir özet çıkarılamadı.")
	}

	items := make([]map[string]interface{}, 0, len(sources))
	for _, s := range sources {
		if strings.Contains(strings.ToLower(s.URL), "wikipedia.org") {
			continue
		}
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

	blocks := []map[string]interface{}{
		{
			"type":    "text",
			"version": 1,
			"data":    map[string]interface{}{"markdown": strings.TrimSpace(b.String())},
		},
	}
	if len(items) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type":    "cards",
			"version": 1,
			"data":    map[string]interface{}{"items": items},
		})
	}
	return blocks
}

func synthesizeWithModel(query string, sources []Source) ([]map[string]interface{}, []Claim, string, error) {
	var b strings.Builder
	b.WriteString("Kullanıcı sorusu: " + query + "\n\n")
	b.WriteString(`Kurallar:
- Sadece verilen kaynaklara dayan. Uydurma.
- Önce soruyu CEVAPLA (kimdir → kim olduğunu anlat; nedir → tanımla). Sadece link listesi YASAK.
- Uzunluk serbest: gerekmeyen tek kelime yazma; karmaşık konuda gerektiği kadar yaz.
- Kullanıcı doğrulama istemediyse "doğrulayamam / ekran arkasındasın" nutku çekme.
- Think, draft, İngilizce analiz, "Here's a thinking process" YASAK.
- Çelişki varsa belirt.
- İsteğe bağlı 0-3 kısa takip sorusu (followups).

`)
	for i, s := range sources {
		body := s.Text
		if body == "" {
			body = s.Snippet
		}
		b.WriteString(fmt.Sprintf("--- KAYNAK %d ---\nURL: %s\nBaşlık: %s\nMetin: %s\n\n",
			i+1, s.URL, s.Title, truncate(body, 2800)))
	}
	b.WriteString(`SADECE geçerli JSON (fence yok, think yok):
{
  "summary": "Türkçe markdown cevap — önce anlatım",
  "followups": ["opsiyonel soru 1", "opsiyonel soru 2"],
  "claims": [
    { "text": "...", "support_urls": ["https://..."], "conflict_urls": [], "confidence": 0.0 }
  ],
  "notes": ""
}
`)

	raw, err := callGroqJSON(b.String())
	if err != nil {
		return nil, nil, "", err
	}

	clean := stripModelNoise(raw)
	if looksLikeLeakedCoT(clean) {
		return nil, nil, "", fmt.Errorf("cot_leak")
	}

	if i := strings.Index(clean, "{"); i >= 0 {
		if j := strings.LastIndex(clean, "}"); j > i {
			clean = clean[i : j+1]
		}
	}

	var parsed struct {
		Summary   string   `json:"summary"`
		Followups []string `json:"followups"`
		Claims    []Claim  `json:"claims"`
		Notes     string   `json:"notes"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, nil, "", fmt.Errorf("parse: %w", err)
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" || looksLikeLeakedCoT(summary) {
		return nil, nil, "", fmt.Errorf("bad summary")
	}

	blocks := []map[string]interface{}{
		{
			"type":    "text",
			"version": 1,
			"data":    map[string]interface{}{"markdown": summary},
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
		if strings.Contains(strings.ToLower(s.URL), "wikipedia.org") {
			continue
		}
		items = append(items, map[string]interface{}{
			"title":       nonEmpty(s.Title, DomainOf(s.URL)),
			"subtitle":    DomainOf(s.URL),
			"description": trimSnippet(nonEmpty(s.Snippet, s.Text), 160),
			"url":         s.URL,
		})
	}
	if len(items) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type":    "cards",
			"version": 1,
			"data":    map[string]interface{}{"items": items},
		})
	}

	if len(parsed.Followups) > 0 {
		opts := make([]map[string]interface{}, 0, 3)
		for i, q := range parsed.Followups {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			opts = append(opts, map[string]interface{}{
				"id":    fmt.Sprintf("f%d", i),
				"label": q,
			})
			if len(opts) >= 3 {
				break
			}
		}
		if len(opts) > 0 {
			blocks = append(blocks, map[string]interface{}{
				"type":    "question",
				"version": 1,
				"data": map[string]interface{}{
					"prompt":  "Devam?",
					"options": opts,
				},
			})
		}
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
				"role": "system",
				"content": `You output ONLY valid JSON. No markdown fences. No <think>. No English chain-of-thought.
Turkish summary answers the user from sources. Length flexible: as short or long as needed. No unsolicited identity policing.`,
			},
			{"role": "user", "content": userContent},
		},
		"temperature": 0.25,
		"max_tokens":  4096,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 90 * time.Second}
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

func stripModelNoise(s string) string {
	s = strings.TrimSpace(s)
	// think etiketleri
	for {
		i := strings.Index(strings.ToLower(s), "<think")
		if i < 0 {
			break
		}
		j := strings.Index(strings.ToLower(s[i:]), "</think>")
		if j < 0 {
			s = strings.TrimSpace(s[:i])
			break
		}
		s = strings.TrimSpace(s[:i] + s[i+j+len("</think>"):])
	}
	return strings.TrimSpace(s)
}

func looksLikeLeakedCoT(s string) bool {
	lower := strings.ToLower(s)
	markers := []string{
		"here's a thinking process",
		"thinking process:",
		"analyze user input",
		"apply persona",
		"drafting response",
		"revised draft",
		"json output only",
		"check against rules",
		"response strategy:",
		"let me draft",
		"internal monologue",
	}
	hits := 0
	for _, m := range markers {
		if strings.Contains(lower, m) {
			hits++
		}
	}
	if hits >= 1 && len(s) > 400 {
		return true
	}
	if hits >= 2 {
		return true
	}
	// aynı cümle aşırı tekrar
	if strings.Count(lower, "kim olduğunu doğrulayamam") > 2 {
		return true
	}
	return false
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
