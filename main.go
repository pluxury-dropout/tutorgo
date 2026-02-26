package main

import (
	"context"
	"log/slog"
	"net/http"

	"tutorgo/config"
	"tutorgo/handlers"
	"tutorgo/logger"

	"github.com/jackc/pgx/v5"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	conn, err := pgx.Connect(context.Background(), cfg.DBUrl)
	if err != nil {
		log.Error("Failed to connect to db", slog.String("error", err.Error()))
	}
	defer conn.Close(context.Background())

	log.Info("Connected to database successfully")

	tutorHandler := handlers.NewTutorHandler(conn, log)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	mux.HandleFunc("/tutors", tutorHandler.Handle)
	mux.HandleFunc("/tutors/{id}", tutorHandler.HandleOne)

	log.Info("Server listening on", slog.String("port", cfg.ServerPort))
	err = http.ListenAndServe(cfg.ServerPort, mux)
	if err != nil {
		log.Error("Server failed", slog.String("error", err.Error()))
	}
}
