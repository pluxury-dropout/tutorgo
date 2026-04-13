package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl      string
	ServerPort string
	JWTSecret  string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	cfg := Config{
		DBUrl:      os.Getenv("DB_URL"),
		ServerPort: os.Getenv("SERVER_PORT"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
	}

	if cfg.DBUrl == "" {
		log.Fatal("DB_URL is required")
	}
	if cfg.ServerPort == "" {
		log.Fatal("SERVER_PORT is required")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	return cfg
}
