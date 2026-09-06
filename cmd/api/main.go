package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/siryan-ai/siryan/internal/config"
	"github.com/siryan-ai/siryan/internal/handlers"
)

func main() {
	cfg := config.Load()

	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	auth := handlers.NewAuthHandler(cfg)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "siryan-api"})
	})

	chat := handlers.NewChatHandler()

	api := r.Group("/api/v1")
	{
		api.POST("/auth/register", auth.Register)
		api.POST("/auth/login", auth.Login)
		api.GET("/auth/me", auth.Me)
		api.POST("/auth/forgot-password", auth.ForgotPassword)
		api.POST("/chat", chat.Chat)
	}

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Println("Siryan API starting on :" + port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
