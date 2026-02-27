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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	return Config{
		DBUrl:      os.Getenv("DB_URL"),
		ServerPort: os.Getenv("SERVER_PORT"),
		JWTSecret:  os.Getenv("JWT_Secret"),
	}
}
