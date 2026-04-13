package router

import (
	"log/slog"

	"tutorgo/config"
	"tutorgo/handlers"
	"tutorgo/middleware"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Setup(pool *pgxpool.Pool, log *slog.Logger, cfg *config.Config) *gin.Engine {
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

	r := gin.New()
	r.Use(gin.Recovery())

	// Public routes
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.Auth(cfg.JWTSecret))
	{
		auth.GET("/tutors/:id", tutorHandler.GetByID)
		auth.PUT("/tutors/:id", tutorHandler.Update)
		auth.DELETE("/tutors/:id", tutorHandler.Delete)

		auth.GET("/students", studentHandler.GetAll)
		auth.POST("/students", studentHandler.Create)
		auth.GET("/students/:id", studentHandler.GetByID)
		auth.PUT("/students/:id", studentHandler.Update)
		auth.DELETE("/students/:id", studentHandler.Delete)

		auth.GET("/courses", courseHandler.GetAll)
		auth.POST("/courses", courseHandler.Create)
		auth.GET("/courses/:id", courseHandler.GetByID)
		auth.PUT("/courses/:id", courseHandler.Update)
		auth.DELETE("/courses/:id", courseHandler.Delete)

		auth.GET("/payments", paymentHandler.GetAll)
		auth.POST("/payments", paymentHandler.Create)
		auth.GET("/payments/balance", paymentHandler.GetBalance)

		auth.GET("/lessons", lessonHandler.GetByCourse)
		auth.POST("/lessons", lessonHandler.Create)
		auth.GET("/lessons/:id", lessonHandler.GetByID)
		auth.PUT("/lessons/:id", lessonHandler.Update)
		auth.DELETE("/lessons/:id", lessonHandler.Delete)
	}

	return r
}
