package config

import (
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	JWTSecret   string
	DBPath      string
	DockerHost  string
	CORSOrigins string
}

func Load() *Config {
	// Load .env file if it exists
	godotenv.Load()

	return &Config{
		Port:        getEnv("PORT", "3000"),
		JWTSecret:   getEnv("JWT_SECRET", "your-secret-key-change-this"),
		DBPath:      getEnv("DB_PATH", "./docode.db"),
		DockerHost:  getEnv("DOCKER_HOST", ""),
		CORSOrigins: getEnv("CORS_ORIGINS", "http://localhost:5173"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
