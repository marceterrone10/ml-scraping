package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv loads variables from a .env file if present.
func LoadEnv() {
	_ = godotenv.Load()
}

// GetEnv returns the value of key or fallback when unset or empty.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
