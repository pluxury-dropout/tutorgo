package router

import (
	"log/slog"
	"net/http"

	"tutorgo/config"
	"tutorgo/handlers"
	"tutorgo/middleware"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Setup(mux *http.ServeMux, pool *pgxpool.Pool, log *slog.Logger, cfg *config.Config) {
	// Repositories
	tutorRepo := repository.NewTutorRepository(pool)
	studentRepo := repository.NewStudentRepository(pool)
	courseRepo := repository.NewCourseRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)
	lessonRepo := repository.NewLessonRepository(pool)

	// Services
	tutorService := service.NewTutorService(tutorRepo)
	studentService := service.NewStudentService(studentRepo)
	courseService := service.NewCourseService(courseRepo, studentRepo, lessonRepo)
	paymentService := service.NewPaymentService(paymentRepo, courseRepo)
	lessonService := service.NewLessonService(lessonRepo, courseRepo)

	// Handlers
	tutorHandler := handlers.NewTutorHandler(tutorService, log)
	authHandler := handlers.NewAuthHandler(tutorService, log, cfg.JWTSecret)
	studentHandler := handlers.NewStudentHandler(studentService, log)
	courseHandler := handlers.NewCourseHandler(courseService, log)
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)
	lessonHandler := handlers.NewLessonHandler(lessonService, log)

	// Routes
	mux.HandleFunc("/auth/register", authHandler.Register)
	mux.HandleFunc("/auth/login", authHandler.Login)

	mux.HandleFunc("/tutors", middleware.Auth(cfg.JWTSecret, tutorHandler.Handle))
	mux.HandleFunc("/tutors/{id}", middleware.Auth(cfg.JWTSecret, tutorHandler.HandleOne))

	mux.HandleFunc("/students", middleware.Auth(cfg.JWTSecret, studentHandler.Handle))
	mux.HandleFunc("/students/{id}", middleware.Auth(cfg.JWTSecret, studentHandler.HandleOne))

	mux.HandleFunc("/courses", middleware.Auth(cfg.JWTSecret, courseHandler.Handle))
	mux.HandleFunc("/courses/{id}", middleware.Auth(cfg.JWTSecret, courseHandler.HandleOne))

	mux.HandleFunc("/payments", middleware.Auth(cfg.JWTSecret, paymentHandler.Handle))
	mux.HandleFunc("/payments/balance", middleware.Auth(cfg.JWTSecret, paymentHandler.GetBalance))

	mux.HandleFunc("/lessons", middleware.Auth(cfg.JWTSecret, lessonHandler.Handle))
	mux.HandleFunc("/lessons/{id}", middleware.Auth(cfg.JWTSecret, lessonHandler.HandleOne))
}
