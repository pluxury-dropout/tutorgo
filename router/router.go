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
	r.POST("/auth/register", gin.WrapF(authHandler.Register))
	r.POST("/auth/login", gin.WrapF(authHandler.Login))

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.Auth(cfg.JWTSecret))
	{
		auth.GET("/tutors", gin.WrapF(tutorHandler.Handle))
		auth.GET("/tutors/:id", gin.WrapF(tutorHandler.HandleOne))
		auth.PUT("/tutors/:id", gin.WrapF(tutorHandler.HandleOne))
		auth.DELETE("/tutors/:id", gin.WrapF(tutorHandler.HandleOne))

		auth.GET("/students", gin.WrapF(studentHandler.Handle))
		auth.POST("/students", gin.WrapF(studentHandler.Handle))
		auth.GET("/students/:id", gin.WrapF(studentHandler.HandleOne))
		auth.PUT("/students/:id", gin.WrapF(studentHandler.HandleOne))
		auth.DELETE("/students/:id", gin.WrapF(studentHandler.HandleOne))

		auth.GET("/courses", gin.WrapF(courseHandler.Handle))
		auth.POST("/courses", gin.WrapF(courseHandler.Handle))
		auth.GET("/courses/:id", gin.WrapF(courseHandler.HandleOne))
		auth.PUT("/courses/:id", gin.WrapF(courseHandler.HandleOne))
		auth.DELETE("/courses/:id", gin.WrapF(courseHandler.HandleOne))

		auth.GET("/payments", gin.WrapF(paymentHandler.Handle))
		auth.POST("/payments", gin.WrapF(paymentHandler.Handle))
		auth.GET("/payments/balance", gin.WrapF(paymentHandler.GetBalance))

		// Lesson handler — нативный Gin
		auth.GET("/lessons", lessonHandler.GetByCourse)
		auth.POST("/lessons", lessonHandler.Create)
		auth.GET("/lessons/:id", lessonHandler.GetByID)
		auth.PUT("/lessons/:id", lessonHandler.Update)
		auth.DELETE("/lessons/:id", lessonHandler.Delete)
	}

	return r
}
