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
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DBUrl)
	if err != nil {
		log.Error("Failed to connect to db", slog.String("error", err.Error()))
	}
	defer pool.Close()

	log.Info("Connected to database successfully")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	tutorRepo := repository.NewTutorRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)
	courseRepo := repository.NewCourseRepository(pool)
	studentRepo := repository.NewStudentRepository(pool)

	tutorService := service.NewTutorService(tutorRepo)
	paymentService := service.NewPaymentService(paymentRepo)
	courseService := service.NewCourseService(courseRepo)
	studentService := service.NewStudentService(studentRepo)

	tutorHandler := handlers.NewTutorHandler(tutorService, log)
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)
	courseHandler := handlers.NewCourseHandler(courseService, log)
	studentHandler := handlers.NewStudentHandler(studentService, log)
	authHandler := handlers.NewAuthHandler(tutorService, log, cfg.JWTSecret)

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
