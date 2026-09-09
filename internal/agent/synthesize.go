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
		return []map[string]interface{}{
			{"type": "text", "version": 1, "data": map[string]interface{}{
				"markdown": "Yeterli açık kaynak bulamadım. Soruyu netleştirip tekrar dene.",
			}},
		}, nil, "no_sources", nil
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
			b.WriteString(t + "\n\n")
		} else {
			b.WriteString("- " + t + "\n")
		}
		n++
		if n >= 4 {
			break
		}
	}
	if b.Len() == 0 {
		b.WriteString("Kaynaklara ulaşıldı ama okunabilir özet çıkarılamadı.")
	}

	items := cardItems(sources)
	blocks := []map[string]interface{}{
		{"type": "text", "version": 1, "data": map[string]interface{}{"markdown": strings.TrimSpace(b.String())}},
	}
	if len(items) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type": "cards", "version": 1, "data": map[string]interface{}{"items": items},
		})
	}
	return blocks
}

func synthesizeWithModel(query string, sources []Source) ([]map[string]interface{}, []Claim, string, error) {
	var b strings.Builder
	b.WriteString("Kullanıcı sorusu: " + query + "\n\n")
	b.WriteString(`Kurallar:
- Sadece verilen kaynaklara dayan. Uydurma.
- Önce soruyu CEVAPLA (kimdir/nedir/konum). Sadece link listesi YASAK.
- Uzunluk serbest: gereksiz kelime yok; gerekirse uzun yaz.
- Kullanıcı doğrulama istemediyse "doğrulayamam" nutku yok.
- Think/draft/İngilizce CoT YASAK.
- Alakasız finans veya rastgele link basma. Bulamadıysan söyle.
- Konum/harita isteği ve kaynakta koordinat veya net yer varsa map doldur; yoksa map:null.
- "URL aç / sayfayı göster" ve URL kaynaklarda geçiyorsa webview doldur. Uydurma URL YASAK. Emin değilsen webview:null.
- Google Maps uygulama linki verme; map block kullan.

`)
	for i, s := range sources {
		body := s.Text
		if body == "" {
			body = s.Snippet
		}
		b.WriteString(fmt.Sprintf("--- KAYNAK %d ---\nURL: %s\nBaşlık: %s\nMetin: %s\n\n",
			i+1, s.URL, s.Title, truncate(body, 2500)))
	}
	b.WriteString(`SADECE JSON:
{
  "summary": "Türkçe markdown cevap",
  "followups": [],
  "claims": [],
  "map": null,
  "webview": null,
  "notes": ""
}
map örneği: {"lat":41.01,"lng":28.97,"zoom":14,"markers":[{"lat":41.01,"lng":28.97,"title":"..."}]}
webview örneği: {"url":"https://..."}
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
		Map       *struct {
			Lat     float64 `json:"lat"`
			Lng     float64 `json:"lng"`
			Zoom    float64 `json:"zoom"`
			Markers []struct {
				Lat   float64 `json:"lat"`
				Lng   float64 `json:"lng"`
				Title string  `json:"title"`
			} `json:"markers"`
		} `json:"map"`
		Webview *struct {
			URL string `json:"url"`
		} `json:"webview"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, nil, "", fmt.Errorf("parse: %w", err)
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" || looksLikeLeakedCoT(summary) {
		return nil, nil, "", fmt.Errorf("bad summary")
	}

	blocks := []map[string]interface{}{
		{"type": "text", "version": 1, "data": map[string]interface{}{"markdown": summary}},
	}

	if parsed.Map != nil && parsed.Map.Lat != 0 && parsed.Map.Lng != 0 {
		zoom := parsed.Map.Zoom
		if zoom == 0 {
			zoom = 14
		}
		markers := []map[string]interface{}{}
		for _, m := range parsed.Map.Markers {
			markers = append(markers, map[string]interface{}{
				"lat": m.Lat, "lng": m.Lng, "title": m.Title,
			})
		}
		if len(markers) == 0 {
			markers = append(markers, map[string]interface{}{
				"lat": parsed.Map.Lat, "lng": parsed.Map.Lng, "title": query,
			})
		}
		blocks = append(blocks, map[string]interface{}{
			"type": "map", "version": 1,
			"data": map[string]interface{}{
				"center":  map[string]interface{}{"lat": parsed.Map.Lat, "lng": parsed.Map.Lng},
				"zoom":    zoom,
				"markers": markers,
			},
		})
	}

	if parsed.Webview != nil {
		u := strings.TrimSpace(parsed.Webview.URL)
		if validURL(u) && urlInSources(u, sources) {
			blocks = append(blocks, map[string]interface{}{
				"type": "webview", "version": 1,
				"data": map[string]interface{}{"url": u},
			})
		}
	}

	if len(parsed.Claims) > 0 {
		rows := [][]string{}
		for _, c := range parsed.Claims {
			rows = append(rows, []string{
				c.Text,
				fmt.Sprintf("%.0f%%", c.Confidence*100),
				strings.Join(c.Support, " "),
			})
		}
		blocks = append(blocks, map[string]interface{}{
			"type": "table", "version": 1,
			"data": map[string]interface{}{
				"caption": "Doğrulama",
				"columns": []string{"İddia", "Güven", "Kaynaklar"},
				"rows":    rows,
			},
		})
	}

	items := cardItems(sources)
	if len(items) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type": "cards", "version": 1, "data": map[string]interface{}{"items": items},
		})
	}

	if len(parsed.Followups) > 0 {
		opts := []map[string]interface{}{}
		for i, q := range parsed.Followups {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			opts = append(opts, map[string]interface{}{"id": fmt.Sprintf("f%d", i), "label": q})
			if len(opts) >= 3 {
				break
			}
		}
		if len(opts) > 0 {
			blocks = append(blocks, map[string]interface{}{
				"type": "question", "version": 1,
				"data": map[string]interface{}{"prompt": "Devam?", "options": opts},
			})
		}
	}

	return blocks, parsed.Claims, parsed.Notes, nil
}

func cardItems(sources []Source) []map[string]interface{} {
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
	return items
}

func validURL(u string) bool {
	u = strings.TrimSpace(u)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	for _, b := range []string{"example.com", "localhost", "127.0.0.1"} {
		if strings.Contains(u, b) {
			return false
		}
	}
	return true
}

func urlInSources(u string, sources []Source) bool {
	for _, s := range sources {
		if s.URL == u || strings.Contains(s.URL, u) || strings.Contains(u, s.URL) {
			return true
		}
		if strings.Contains(s.Text, u) || strings.Contains(s.Snippet, u) {
			return true
		}
	}
	return false
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
				"content": `ONLY valid JSON. No fences. No <think>. No English CoT.
Turkish summary from sources. Flexible length. No fake URLs. No unsolicited identity policing.
Use map/webview only when justified by sources.`,
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
	client := &http.Client{Timeout: 45 * time.Second}
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
		"here's a thinking process", "thinking process:", "analyze user input",
		"apply persona", "drafting response", "revised draft", "json output only",
		"check against rules", "response strategy:", "let me draft",
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
