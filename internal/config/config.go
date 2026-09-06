// internal/config/config.go
package config

import (
	"os"
)

type Config struct {
	Port               string
	GinMode            string
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseServiceKey string
}

func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		GinMode:            getEnv("GIN_MODE", "release"),
		SupabaseURL:        getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:    getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceKey: getEnv("SUPABASE_SERVICE_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
