package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// NeedsExternalData: "araştır" kelimesi şart değil.
func NeedsExternalData(userMessage string) bool {
	msg := strings.ToLower(strings.TrimSpace(userMessage))
	if msg == "" {
		return false
	}

	// Hızlı sezgisel — model yoksa bile
	triggers := []string{
		"bul ", "bul.", "fiyat", "puan", "trendyol", "hepsiburada", "n11",
		"en iyi", "karşılaştır", "karsilastir", "güncel", "guncel", "bugün",
		"bugun", "2024", "2025", "2026", "nerede", "konum", "harita",
		"kimdir", "nedir", "ne kadar", "stok", "satın", "satin",
		"araştır", "arastir", "research", "haber", "son durum",
		"kaç", "kac tl", "tl altı", "tl alti",
	}
	for _, t := range triggers {
		if strings.Contains(msg, t) {
			return true
		}
	}

	// Model router (opsiyonel, başarısızsa sezgisel yeterli)
	if v, err := modelRouter(userMessage); err == nil {
		return v
	}
	return false
}

func modelRouter(userMessage string) (bool, error) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return false, errNoKey
	}
	payload := map[string]interface{}{
		"model": "qwen/qwen3.6-27b",
		"messages": []map[string]string{
			{"role": "system", "content": `Kullanıcı mesajı için dış dünya verisi (web, ürün, fiyat, güncel olay, konum) gerekli mi?
SADECE JSON: {"need_external": true} veya {"need_external": false}
Selamlaşma, duygu, genel sohbet, kod yazımı (genel) → false
Ürün arama, fiyat, güncel bilgi, spesifik gerçek, karşılaştırma → true`},
			{"role": "user", "content": userMessage},
		},
		"temperature": 0,
		"max_tokens":  40,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return false, errNoKey
	}
	var gr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &gr); err != nil || len(gr.Choices) == 0 {
		return false, errNoKey
	}
	c := gr.Choices[0].Message.Content
	return strings.Contains(c, "true"), nil
}

var errNoKey = errString("router unavailable")

type errString string

func (e errString) Error() string { return string(e) }
