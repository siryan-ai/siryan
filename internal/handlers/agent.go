package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/siryan-ai/siryan/internal/agent"
)

type AgentHandler struct{}

func NewAgentHandler() *AgentHandler { return &AgentHandler{} }

func (h *AgentHandler) Research(c *gin.Context) {
	var req agent.ResearchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query gerekli"})
		return
	}

	result, err := agent.RunResearch(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   result.Query,
		"sources": result.Sources,
		"claims":  result.Claims,
		"blocks":  result.Blocks,
		"notes":   result.Notes,
		// Flutter chat ile aynı dil
		"content": firstText(result.Blocks),
	})
}

func firstText(blocks []map[string]interface{}) string {
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
