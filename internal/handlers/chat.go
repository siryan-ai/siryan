// internal/handlers/chat.go

package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/siryan-ai/siryan/internal/agent"
	"github.com/siryan-ai/siryan/internal/chatutil"
	"github.com/siryan-ai/siryan/internal/prompt"
)

type ChatHandler struct {
	client *http.Client
}

func NewChatHandler() *ChatHandler {
	return &ChatHandler{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages []chatMessage `json:"messages" binding:"required"`
	Model    string        `json:"model"`
	Mode     string        `json:"mode"` // empathy | critical
}

func looksLikeResearch(msgs []chatMessage) (string, bool) {
	if len(msgs) == 0 {
		return "", false
	}
	q := strings.ToLower(strings.TrimSpace(msgs[len(msgs)-1].Content))
	if q == "" {
		return "", false
	}
	keys := []string{
		"araştır",
		"arastir",
		"research",
		"güncel",
		"guncel",
		"webde",
		"internetten",
		"kaynak bul",
		"doğrula",
		"dogrula",
		"son durum",
		"haberler",
		"haber ",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return msgs[len(msgs)-1].Content, true
		}
	}
	return "", false
}

func firstTextFromBlocks(blocks []map[string]interface{}) string {
	for _, b := range blocks {
		if b["type"] == "text" {
			if data, ok := b["data"].(map[string]interface{}); ok {
				if md, ok := data["markdown"].(string); ok {
					return md
				}
			}
		}
	}
	return "Araştırma tamamlandı"
}

func (h *ChatHandler) Chat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek"})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages boş olamaz"})
		return
	}

	mode := req.Mode
	if mode != "empathy" {
		mode = "critical"
	}

	// Research agent köprüsü
	if query, ok := looksLikeResearch(req.Messages); ok {
		result, err := agent.RunResearch(c.Request.Context(), agent.ResearchRequest{
			Query:   query,
			Locale:  "tr",
			MaxURLs: 4,
		})
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":       "research",
			"model":    "agent-research",
			"mode":     mode,
			"content":  firstTextFromBlocks(result.Blocks),
			"thinking": "",
			"blocks":   result.Blocks,
			"sources":  result.Sources,
			"notes":    result.Notes,
		})
		return
	}

	model := req.Model
	if model == "" {
		model = "qwen/qwen3.6-27b"
	}

	system := prompt.SystemForMode(mode)
	messages := []chatMessage{{Role: "system", Content: system}}
	messages = append(messages, req.Messages...)

	// Temperature değerini standart if/else bloğu ile belirliyoruz
	temperature := 0.4
	if mode == "empathy" {
		temperature = 0.6
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  3072,
	}
	body, _ := json.Marshal(payload)

	var respBody []byte
	var status int

	for attempt := 0; attempt < 2; attempt++ {
		httpReq, err := http.NewRequest(
			http.MethodPost,
			"https://api.groq.com/openai/v1/chat/completions",
			bytes.NewReader(body),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "istek oluşturulamadı"})
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))

		resp, err := h.client.Do(httpReq)
		if err != nil {
			time.Sleep(400 * time.Millisecond)
			continue
		}

		respBody, _ = io.ReadAll(resp.Body)
		status = resp.StatusCode
		resp.Body.Close()

		if status == 429 || status >= 500 {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	if respBody == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "model servisine ulaşılamadı"})
		return
	}
	if status != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "model hatası",
			"status": status,
		})
		return
	}

	var groqResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage interface{} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &groqResp); err != nil || len(groqResp.Choices) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "geçersiz model cevabı"})
		return
	}

	raw := groqResp.Choices[0].Message.Content
	_, thinking := chatutil.SplitThinking(raw)
	blocks, content, ok := chatutil.ParseBlocks(raw)
	if !ok || content == "" {
		content = "Cevap alınamadı"
		if len(blocks) == 0 {
			blocks = []chatutil.Block{{
				Type:    "text",
				Version: 1,
				Data:    map[string]interface{}{"markdown": content},
			}}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       groqResp.ID,
		"model":    groqResp.Model,
		"mode":     mode,
		"content":  content,
		"thinking": thinking,
		"blocks":   blocks,
		"usage":    groqResp.Usage,
	})
}
