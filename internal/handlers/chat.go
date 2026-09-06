package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/siryan-ai/siryan/internal/chatutil"
	"github.com/siryan-ai/siryan/internal/prompt"
)

type ChatHandler struct{}

func NewChatHandler() *ChatHandler { return &ChatHandler{} }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages []chatMessage `json:"messages" binding:"required"`
	Model    string        `json:"model"`
}

type chatResponse struct {
	ID       string      `json:"id"`
	Model    string      `json:"model"`
	Content  string      `json:"content"`
	Thinking string      `json:"thinking,omitempty"`
	Usage    interface{} `json:"usage,omitempty"`
}

func (h *ChatHandler) Chat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	model := req.Model
	if model == "" {
		model = "qwen/qwen3.6-27b"
	}

	system := prompt.Build(prompt.PromptInput{
		User: prompt.UserContext{
			Language: "tr",
		},
		Session: prompt.SessionContext{
			Model:    model,
			Platform: "flutter",
		},
	})

	messages := []chatMessage{
		{Role: "system", Content: system},
	}
	messages = append(messages, req.Messages...)

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  2048,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest(
		http.MethodPost,
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request failed"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "groq unreachable"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.Data(resp.StatusCode, "application/json", respBody)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid groq response"})
		return
	}

	raw := groqResp.Choices[0].Message.Content
	content, thinking := chatutil.SplitThinking(raw)

	c.JSON(http.StatusOK, chatResponse{
		ID:       groqResp.ID,
		Model:    groqResp.Model,
		Content:  content,
		Thinking: thinking,
		Usage:    groqResp.Usage,
	})
}
