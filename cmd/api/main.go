// cmd/api/main.go
package main

import (
	"log"

	"github.com/efeca/siryan-api/internal/config"
	"github.com/efeca/siryan-api/internal/handlers"
	"github.com/gin-gonic/gin"
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

	api := r.Group("/api/v1")
	{
		api.POST("/auth/register", auth.Register)
		api.POST("/auth/login", auth.Login)
		api.GET("/auth/me", auth.Me)
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
