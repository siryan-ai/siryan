package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/siryan-ai/siryan/internal/chatutil"
	"github.com/siryan-ai/siryan/internal/prompt"
)

type ChatHandler struct {
	client *http.Client
}

func NewChatHandler() *ChatHandler {
	return &ChatHandler{
		client: &http.Client{Timeout: 45 * time.Second},
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

type chatResponse struct {
	ID       string      `json:"id"`
	Model    string      `json:"model"`
	Mode     string      `json:"mode"`
	Content  string      `json:"content"`
	Thinking string      `json:"thinking,omitempty"`
	Usage    interface{} `json:"usage,omitempty"`
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

	model := req.Model
	if model == "" {
		model = "qwen/qwen3.6-27b"
	}

	system := prompt.SystemForMode(mode)
	messages := []chatMessage{{Role: "system", Content: system}}
	messages = append(messages, req.Messages...)

	// Go dilinde ternary operator (? :) olmadığı için if/else kullanıyoruz
	temperature := 0.4
	if mode == "empathy" {
		temperature = 0.6
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  512, // kısa tut
	}
	body, _ := json.Marshal(payload)

	var respBody []byte
	var status int
	var lastErr error

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
			lastErr = err
			time.Sleep(400 * time.Millisecond)
			continue
		}

		respBody, _ = io.ReadAll(resp.Body)
		status = resp.StatusCode
		resp.Body.Close()

		// retry only transient
		if status == 429 || status >= 500 {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	_ = lastErr // derleyici uyarısını önlemek için

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
	content, thinking := chatutil.SplitThinking(raw)
	if content == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "boş cevap"})
		return
	}

	c.JSON(http.StatusOK, chatResponse{
		ID:       groqResp.ID,
		Model:    groqResp.Model,
		Mode:     mode,
		Content:  content,
		Thinking: thinking,
		Usage:    groqResp.Usage,
	})
}
