package router

import (
	"log/slog"

	"tutorgo/config"
	"tutorgo/handlers"
	"tutorgo/middleware"
	"tutorgo/repository"
	"tutorgo/service"

	"github.com/gin-contrib/cors"
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
	enrollmentRepo := repository.NewEnrollmentRepository(pool)
	attendanceRepo := repository.NewAttendanceRepository(pool)

	// Services
	tutorService := service.NewTutorService(tutorRepo)
	studentService := service.NewStudentService(studentRepo)
	courseService := service.NewCourseService(courseRepo, studentRepo, lessonRepo)
	paymentService := service.NewPaymentService(paymentRepo, courseRepo)
	lessonService := service.NewLessonService(lessonRepo, courseRepo)
	enrollmentService := service.NewEnrollmentService(enrollmentRepo, courseRepo, studentRepo)
	attendanceService := service.NewAttendanceService(attendanceRepo, lessonRepo, courseRepo)

	// Handlers
	tutorHandler := handlers.NewTutorHandler(tutorService, log)
	authHandler := handlers.NewAuthHandler(tutorService, log, cfg.JWTSecret)
	studentHandler := handlers.NewStudentHandler(studentService, log)
	courseHandler := handlers.NewCourseHandler(courseService, log)
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)
	lessonHandler := handlers.NewLessonHandler(lessonService, log)
	enrollmentHandler := handlers.NewEnrollmentHandler(enrollmentService, log)
	attendanceHandler := handlers.NewAttendanceHandler(attendanceService, log)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Public routes
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.Auth(cfg.JWTSecret))
	{
		auth.GET("/tutors/:id", tutorHandler.GetByID)
		auth.PUT("/tutors/:id", tutorHandler.Update)
		auth.PUT("/tutors/:id/password", tutorHandler.ChangePassword)
		auth.DELETE("/tutors/:id", tutorHandler.Delete)

		auth.GET("/students", studentHandler.GetAll)
		auth.POST("/students", studentHandler.Create)
		auth.GET("/students/:id", studentHandler.GetByID)
		auth.PUT("/students/:id", studentHandler.Update)
		auth.DELETE("/students/:id", studentHandler.Delete)
		auth.GET("/students/:id/courses", courseHandler.GetByStudent)

		auth.GET("/courses", courseHandler.GetAll)
		auth.POST("/courses", courseHandler.Create)
		auth.GET("/courses/:id", courseHandler.GetByID)
		auth.PUT("/courses/:id", courseHandler.Update)
		auth.DELETE("/courses/:id", courseHandler.Delete)

		auth.GET("/payments", paymentHandler.GetAll)
		auth.POST("/payments", paymentHandler.Create)
		auth.GET("/payments/recent", paymentHandler.GetRecent)
		auth.GET("/payments/balance", paymentHandler.GetBalance)
		auth.GET("/payments/monthly-income", paymentHandler.GetMonthlyIncome)

		auth.GET("/lessons", lessonHandler.GetByCourse)
		auth.POST("/lessons", lessonHandler.Create)
		auth.GET("/lessons/:id", lessonHandler.GetByID)
		auth.PUT("/lessons/:id", lessonHandler.Update)
		auth.DELETE("/lessons/:id", lessonHandler.Delete)

		auth.GET("/calendar", lessonHandler.GetCalendar)

		auth.GET("/courses/:id/enrollments", enrollmentHandler.GetByCourse)
		auth.POST("/courses/:id/enrollments", enrollmentHandler.Add)
		auth.DELETE("/courses/:id/enrollments/:studentId", enrollmentHandler.Remove)

		auth.GET("/lessons/:id/attendance", attendanceHandler.Get)
		auth.PUT("/lessons/:id/attendance", attendanceHandler.Update)
	}

	return r
}
