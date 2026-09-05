package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	GinMode            string
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseServiceKey string
	RedisURL           string
	RedisToken         string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		Port:               getEnv("PORT", "8080"),
		GinMode:            getEnv("GIN_MODE", "debug"),
		SupabaseURL:        getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:    getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceKey: getEnv("SUPABASE_SERVICE_KEY", ""),
		RedisURL:           getEnv("UPSTASH_REDIS_REST_URL", ""),
		RedisToken:         getEnv("UPSTASH_REDIS_REST_TOKEN", ""),
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
