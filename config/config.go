package config

import (
	"log/slog"
	"os"
	"strconv"

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
	Env              string
	S3Endpoint       string
	S3Region         string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	ResendAPIKey     string
	EmailFrom        string
	AppURL           string
	RedisURL         string
	DBMaxConns       int32
}

// envInt32 читает положительное целое из env. Мусор и неположительные значения
// не роняют старт: сервис поднимется на дефолте, но скажет об этом в лог —
// опечатка в переменной не повод не пустить людей на урок.
func envInt32(key string, def int32, log *slog.Logger) int32 {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		log.Warn("invalid env value, using default",
			slog.String("key", key), slog.String("value", raw), slog.Int("default", int(def)))
		return def
	}
	return int32(v)
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
		Env:              os.Getenv("APP_ENV"),
		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
		S3Region:         os.Getenv("S3_REGION"),
		S3AccessKey:      os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:      os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		ResendAPIKey:     os.Getenv("RESEND_API_KEY"),
		EmailFrom:        os.Getenv("EMAIL_FROM"),
		// Отдельно от ALLOWED_ORIGIN: тот — список источников для CORS, а этот —
		// один адрес, на который ведут ссылки в письмах.
		AppURL: os.Getenv("APP_URL"),
		// Пустой RedisURL — валидный однопроцессный режим, не ошибка. См. pubsub.New.
		RedisURL: os.Getenv("REDIS_URL"),
		// Размер пула — env, а не константа: у ролей разный профиль (API держит
		// много коротких транзакций, воркер — несколько долгих соединений), образ
		// один, а крутить значение приходится по показаниям /health.
		DBMaxConns: envInt32("DB_MAX_CONNS", 7, log),
	}

	if cfg.EmailFrom == "" {
		cfg.EmailFrom = "TutorHub <onboarding@resend.dev>" // ponytail: дефолт для dev/resend-песочницы
	}
	if cfg.AppURL == "" {
		cfg.AppURL = "https://amida.kz"
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
