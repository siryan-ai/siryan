package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
s
	"github.com/efeca/siryan-api/internal/config"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

type registerRequest struct {
	Email    string  `json:"email" binding:"required,email"`
	Password string  `json:"password" binding:"required,min=6"`
	Phone    *string `json:"phone"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := map[string]interface{}{
		"email":    req.Email,
		"password": req.Password,
	}
	if req.Phone != nil && *req.Phone != "" {
		payload["phone"] = *req.Phone
	}

	body, _ := json.Marshal(payload)

	url := h.cfg.SupabaseURL + "/auth/v1/signup"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request failed"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", h.cfg.SupabaseAnonKey)
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.SupabaseAnonKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "supabase unreachable"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	c.Data(resp.StatusCode, "application/json", respBody)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload := map[string]string{
		"email":    req.Email,
		"password": req.Password,
	}
	body, _ := json.Marshal(payload)

	url := h.cfg.SupabaseURL + "/auth/v1/token?grant_type=password"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request failed"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", h.cfg.SupabaseAnonKey)
	httpReq.Header.Set("Authorization", "Bearer "+h.cfg.SupabaseAnonKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "supabase unreachable"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	c.Data(resp.StatusCode, "application/json", respBody)
}

func (h *AuthHandler) Me(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	url := h.cfg.SupabaseURL + "/auth/v1/user"
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "request failed"})
		return
	}

	httpReq.Header.Set("apikey", h.cfg.SupabaseAnonKey)
	httpReq.Header.Set("Authorization", token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "supabase unreachable"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", respBody)
}
