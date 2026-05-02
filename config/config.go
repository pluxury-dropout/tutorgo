package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl           string
	ServerPort      string
	JWTSecret       string
	AllowedOrigin   string
	LiveKitURL      string
	LiveKitAPIKey   string
	LiveKitAPISecret string
}

func Load() Config {
	godotenv.Load() // optional: ignored in production where env vars are already set

	port := os.Getenv("SERVER_PORT")
	if port != "" && port[0] != ':' {
		port = ":" + port
	}

	cfg := Config{
		DBUrl:            os.Getenv("DB_URL"),
		ServerPort:       port,
		JWTSecret:        os.Getenv("JWT_SECRET"),
		AllowedOrigin:    os.Getenv("ALLOWED_ORIGIN"),
		LiveKitURL:       os.Getenv("LIVEKIT_URL"),
		LiveKitAPIKey:    os.Getenv("LIVEKIT_API_KEY"),
		LiveKitAPISecret: os.Getenv("LIVEKIT_API_SECRET"),
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
