package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Error("failed to ping db", slog.String("error", err.Error()))
		os.Exit(1)
	}

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
	lessonRepo := repository.NewLessonRepository(pool)

	tutorService := service.NewTutorService(tutorRepo)
	paymentService := service.NewPaymentService(paymentRepo)
	courseService := service.NewCourseService(courseRepo)
	studentService := service.NewStudentService(studentRepo)
	lessonService := service.NewLessonService(lessonRepo)

	tutorHandler := handlers.NewTutorHandler(tutorService, log)
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)
	courseHandler := handlers.NewCourseHandler(courseService, log)
	studentHandler := handlers.NewStudentHandler(studentService, log)
	authHandler := handlers.NewAuthHandler(tutorService, log, cfg.JWTSecret)
	lessonHandler := handlers.NewLessonHandler(lessonService, log)

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

	mux.HandleFunc("/lessons", middleware.Auth(cfg.JWTSecret, lessonHandler.Handle))
	mux.HandleFunc("/lessons/{id}", middleware.Auth(cfg.JWTSecret, lessonHandler.HandleOne))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "database unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: mux,
	}

	go func() {
		log.Info("Server is listening on", slog.String("port", cfg.ServerPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed", slog.String("error", err.Error()))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("Server exited cleanly")

}
