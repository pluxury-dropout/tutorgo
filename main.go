package main

import (
	"context"
	"log/slog"
	"net/http"

	"tutorgo/config"
	"tutorgo/handlers"
	"tutorgo/logger"
	"tutorgo/middleware"
	"tutorgo/repository"

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

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	tutorRepo := repository.NewTutorRepository(conn)
	paymentRepo := repository.NewPaymentRepository(conn)
	courseRepo := repository.NewCourseRepository(conn)
	studentRepo := repository.NewStudentRepository(conn)

	courseHandler := handlers.NewCourseHandler(courseRepo, log)
	studentHandler := handlers.NewStudentHandler(studentRepo, log)
	tutorHandler := handlers.NewTutorHandler(tutorRepo, log)
	authHandler := handlers.NewAuthHandler(tutorRepo, log, cfg.JWTSecret)
	paymentHandler := handlers.NewPaymentHandler(paymentRepo, log)

	mux.HandleFunc("/payments", middleware.Auth(cfg.JWTSecret, paymentHandler.Handle))
	mux.HandleFunc("/payments/balance", middleware.Auth(cfg.JWTSecret, paymentHandler.GetBalance))

	mux.HandleFunc("/courses", middleware.Auth(cfg.JWTSecret, courseHandler.Handle))
	mux.HandleFunc("/courses/{id}", middleware.Auth(cfg.JWTSecret, courseHandler.HandleOne))

	mux.HandleFunc("/students", middleware.Auth(cfg.JWTSecret, studentHandler.Handle))
	mux.HandleFunc("/students/{id}", middleware.Auth(cfg.JWTSecret, studentHandler.HandleOne))

	mux.HandleFunc("/auth/register", authHandler.Register)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.HandleFunc("/tutors", middleware.Auth(cfg.JWTSecret, tutorHandler.Handle))
	mux.HandleFunc("/tutors/{id}", middleware.Auth(cfg.JWTSecret, tutorHandler.HandleOne))

	log.Info("Server listening on", slog.String("port", cfg.ServerPort))
	err = http.ListenAndServe(cfg.ServerPort, mux)
	if err != nil {
		log.Error("Server failed", slog.String("error", err.Error()))
	}
}
