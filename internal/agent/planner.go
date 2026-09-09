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

type SearchPlan struct {
	Queries []string `json:"queries"`
	Intent  string   `json:"intent"`
}

func PlanQueries(userMessage string) SearchPlan {
	if plan, err := planWithModel(userMessage); err == nil && len(plan.Queries) > 0 {
		if len(plan.Queries) > 3 {
			plan.Queries = plan.Queries[:3]
		}
		return plan
	}
	return heuristicPlan(userMessage)
}

func heuristicPlan(msg string) SearchPlan {
	msg = strings.TrimSpace(msg)
	lower := strings.ToLower(msg)
	intent := "general"
	if strings.Contains(lower, "trendyol") || strings.Contains(lower, "fiyat") ||
		strings.Contains(lower, "puan") || strings.Contains(lower, "kulaklık") {
		intent = "product_search"
	}
	if strings.Contains(lower, "nerede") || strings.Contains(lower, "konum") ||
		strings.Contains(lower, "harita") {
		intent = "place"
	}

	var qs []string
	switch intent {
	case "product_search":
		qs = []string{msg, msg + " site:trendyol.com", msg + " inceleme"}
	case "place":
		qs = []string{msg + " konum", msg + " adresi"}
	default:
		qs = []string{msg, msg + " nedir"}
	}
	if len(qs) > 3 {
		qs = qs[:3]
	}
	return SearchPlan{Queries: qs, Intent: intent}
}

func planWithModel(userMessage string) (SearchPlan, error) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return SearchPlan{}, fmt.Errorf("no key")
	}
	sys := `Kullanıcı isteği için 2 veya 3 KISA arama sorgusu üret.
Ham cümleyi tek sorgu yapma. Ürün/fiyat/konum için odaklı sorgular.
JSON only: {"intent":"product_search|fact|place|news|general","queries":["q1","q2"]}`

	payload := map[string]interface{}{
		"model": "qwen/qwen3.6-27b",
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": userMessage},
		},
		"temperature": 0.2,
		"max_tokens":  200,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return SearchPlan{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return SearchPlan{}, fmt.Errorf("groq %d", resp.StatusCode)
	}
	var gr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &gr); err != nil || len(gr.Choices) == 0 {
		return SearchPlan{}, fmt.Errorf("bad body")
	}
	c := gr.Choices[0].Message.Content
	if i := strings.Index(c, "{"); i >= 0 {
		if j := strings.LastIndex(c, "}"); j > i {
			c = c[i : j+1]
		}
	}
	var plan SearchPlan
	if err := json.Unmarshal([]byte(c), &plan); err != nil {
		return SearchPlan{}, err
	}
	if len(plan.Queries) == 0 {
		return SearchPlan{}, fmt.Errorf("no queries")
	}
	if len(plan.Queries) > 3 {
		plan.Queries = plan.Queries[:3]
	}
	return plan, nil
}
