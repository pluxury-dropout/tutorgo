package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl            string
	ServerPort       string
	JWTSecret        string
	AllowedOrigin    string
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string
}

func Load(log *slog.Logger) Config {
	godotenv.Load()

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
		log.Error("DB_URL is required")
		os.Exit(1)
	}
	if cfg.ServerPort == "" {
		log.Error("SERVER_PORT is required")
		os.Exit(1)
	}
	if cfg.JWTSecret == "" {
		log.Error("JWT_SECRET is required")
		os.Exit(1)
	}

	return cfg
}
