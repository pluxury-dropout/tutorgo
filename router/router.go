package router

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"tutorgo/config"
	"tutorgo/email"
	"tutorgo/handlers"
	"tutorgo/middleware"
	"tutorgo/repository"
	"tutorgo/service"
	"tutorgo/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

func Setup(pool *pgxpool.Pool, log *slog.Logger, cfg *config.Config) (*gin.Engine, service.SubscriptionService) {
	// Repositories
	tutorRepo := repository.NewTutorRepository(pool)
	refreshTokenRepo := repository.NewRefreshTokenRepository(pool)
	studentRepo := repository.NewStudentRepository(pool)
	courseRepo := repository.NewCourseRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)
	lessonRepo := repository.NewLessonRepository(pool)
	enrollmentRepo := repository.NewEnrollmentRepository(pool)
	attendanceRepo := repository.NewAttendanceRepository(pool)
	taskRepo := repository.NewTaskRepository(pool)
	whiteboardRepo := repository.NewWhiteboardRepository(pool)
	subscriptionRepo := repository.NewSubscriptionRepository(pool)
	studentRefreshRepo := repository.NewStudentRefreshTokenRepository(pool)
	pendingRepo := repository.NewPendingRegistrationRepository(pool)
	materialRepo := repository.NewMaterialRepository(pool)

	// Services
	tutorService := service.NewTutorService(tutorRepo, subscriptionRepo, pool)
	emailSender := email.NewSender(cfg.ResendAPIKey, cfg.EmailFrom, log)
	registrationService := service.NewRegistrationService(pendingRepo, tutorService, emailSender)
	refreshTokenService := service.NewRefreshTokenService(refreshTokenRepo)
	studentService := service.NewStudentService(studentRepo, paymentRepo)
	studentRefreshService := service.NewStudentRefreshTokenService(studentRefreshRepo)
	courseService := service.NewCourseService(courseRepo, studentRepo)
	paymentService := service.NewPaymentService(paymentRepo, courseRepo)
	lessonService := service.NewLessonService(lessonRepo, courseRepo, paymentRepo)
	enrollmentService := service.NewEnrollmentService(enrollmentRepo, courseRepo, studentRepo)
	attendanceService := service.NewAttendanceService(attendanceRepo, lessonRepo, courseRepo)
	taskService := service.NewTaskService(taskRepo)
	whiteboardService := service.NewWhiteboardService(whiteboardRepo)
	subscriptionService := service.NewSubscriptionService(subscriptionRepo, service.StubProvider{})
	materialService := service.NewMaterialService(materialRepo)

	// Handlers
	tutorHandler := handlers.NewTutorHandler(tutorService, refreshTokenService, log)
	authHandler := handlers.NewAuthHandler(tutorService, registrationService, refreshTokenService, log, cfg.JWTSecret, cfg.Env == "production")
	studentHandler := handlers.NewStudentHandler(studentService, log)
	courseHandler := handlers.NewCourseHandler(courseService, log)
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)
	lessonHandler := handlers.NewLessonHandler(lessonService, log)
	enrollmentHandler := handlers.NewEnrollmentHandler(enrollmentService, log)
	attendanceHandler := handlers.NewAttendanceHandler(attendanceService, log)
	taskHandler := handlers.NewTaskHandler(taskService, log)
	callHandler := handlers.NewCallHandler(lessonService, log, cfg.LiveKitURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret, studentService)
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService, log)
	studentAuthHandler := handlers.NewStudentAuthHandler(studentService, studentRefreshService, log, cfg.JWTSecret, cfg.Env == "production")

	origins := []string{"http://localhost:3000"}
	if cfg.AllowedOrigin != "" {
		origins = append(origins, cfg.AllowedOrigin)
	}
	store, err := storage.New(context.Background(), *cfg)
	if err != nil {
		log.Error("failed to init object storage", "err", err)
		os.Exit(1)
	}
	wbHubManager := handlers.NewWbHubManager(whiteboardService, subscriptionService, log, cfg.JWTSecret, origins)
	whiteboardHandler := handlers.NewWhiteboardHandler(whiteboardService, log, wbHubManager, store, studentService)
	materialHandler := handlers.NewMaterialHandler(materialService, store, log)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(log))
	r.Use(func(c *gin.Context) {
		// JSON-тела — 1 МБ; загрузки файлов (multipart) — 50 МБ, точный размер
		// проверяет сам хендлер по header.Size.
		limit := int64(1 << 20)
		if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			limit = 50 << 20
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	})
	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Public routes
	authLimiter := middleware.RateLimit(rate.Every(12*time.Second), 3)
	// Регистрация/resend шлют письма — жёстче лимит по IP (≈5/час), беречь sender-репутацию.
	regLimiter := middleware.RateLimit(rate.Every(12*time.Minute), 5)
	r.POST("/auth/register", regLimiter, authHandler.Register)
	r.POST("/auth/register/verify", authLimiter, authHandler.RegisterVerify)
	r.POST("/auth/register/resend", regLimiter, authHandler.RegisterResend)
	r.POST("/auth/login", authLimiter, authHandler.Login)
	r.POST("/auth/refresh", authLimiter, authHandler.Refresh)
	r.POST("/auth/logout", authLimiter, authHandler.Logout)
	r.GET("/public/lessons/:id/room-status", callHandler.GetRoomStatus)
	r.GET("/public/quick/:id/status", callHandler.GetQuickRoomStatus)
	r.GET("/public/quick/:id/guest-token", middleware.RateLimit(rate.Every(3*time.Second), 5), callHandler.GetQuickGuestToken)
	r.POST("/webhooks/livekit", callHandler.LiveKitWebhook)
	r.POST("/subscription/webhook", subscriptionHandler.Webhook)

	// Public student auth routes
	studentAuthLimiter := middleware.RateLimit(rate.Every(12*time.Second), 3)
	r.POST("/student/auth/accept-invite", studentAuthLimiter, studentAuthHandler.AcceptInvite)
	r.POST("/student/auth/login", studentAuthLimiter, studentAuthHandler.Login)
	r.POST("/student/auth/refresh", studentAuthLimiter, studentAuthHandler.Refresh)
	r.POST("/student/auth/logout", studentAuthHandler.Logout)

	// Whiteboard public routes (no JWT required)
	r.GET("/public/board/join/:token", whiteboardHandler.JoinByInvite)
	r.GET("/public/board-assets/:id", whiteboardHandler.ServeAsset)
	// Гостевая заливка картинки (Ctrl+V ученика). Публично → rate-limit против
	// абьюза: invite-ссылка расшариваема. Только image-MIME (в хендлере).
	guestAssetLimiter := middleware.RateLimit(rate.Every(3*time.Second), 5)
	r.POST("/public/board/:token/assets", guestAssetLimiter, whiteboardHandler.UploadAssetByInvite)
	r.GET("/ws/board/:pageId", wbHubManager.ServeWS(whiteboardService))

	// open — авторизовано, но доступно даже при истёкшей подписке (чтобы заплатить)
	open := r.Group("/")
	open.Use(middleware.Auth(cfg.JWTSecret))
	{
		open.GET("/subscription", subscriptionHandler.GetStatus)
		open.POST("/subscription/checkout", subscriptionHandler.Checkout)
		open.POST("/subscription/cancel", subscriptionHandler.Cancel)
		open.POST("/subscription/change-plan", subscriptionHandler.ChangePlan)
		open.GET("/tutors/:id", tutorHandler.GetByID)
		open.PUT("/tutors/:id", tutorHandler.Update)
	}

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.Auth(cfg.JWTSecret))
	auth.Use(middleware.RequireActiveSubscription(subscriptionService))
	{
		auth.PUT("/tutors/:id/password", tutorHandler.ChangePassword)
		auth.DELETE("/tutors/:id", tutorHandler.Delete)

		auth.GET("/students", studentHandler.GetAll)
		auth.POST("/students", studentHandler.Create)
		auth.GET("/students/:id", studentHandler.GetByID)
		auth.PUT("/students/:id", studentHandler.Update)
		auth.DELETE("/students/:id", studentHandler.Delete)
		auth.GET("/students/:id/courses", courseHandler.GetByStudent)
		auth.POST("/students/:id/invite", studentHandler.Invite)

		auth.GET("/courses", courseHandler.GetAll)
		auth.POST("/courses", courseHandler.Create)
		auth.GET("/courses/archived", courseHandler.GetArchived)
		auth.GET("/courses/:id", courseHandler.GetByID)
		auth.PUT("/courses/:id", courseHandler.Update)
		auth.DELETE("/courses/:id", courseHandler.Delete)
		auth.POST("/courses/:id/restore", courseHandler.Restore)
		auth.GET("/courses/:id/homework", courseHandler.GetHomework)
		auth.PUT("/courses/:id/homework", courseHandler.UpdateHomework)

		auth.GET("/payments", paymentHandler.GetAll)
		auth.POST("/payments", paymentHandler.Create)
		auth.PUT("/payments/:id", paymentHandler.Update)
		auth.DELETE("/payments/:id", paymentHandler.Delete)
		auth.GET("/payments/recent", paymentHandler.GetRecent)
		auth.GET("/payments/balance", paymentHandler.GetBalance)
		auth.GET("/payments/monthly-income", paymentHandler.GetMonthlyIncome)
		auth.GET("/payments/monthly-expected", paymentHandler.GetMonthlyExpected)

		auth.GET("/lessons", lessonHandler.GetByCourse)
		auth.POST("/lessons", lessonHandler.Create)
		auth.POST("/lessons/bulk", lessonHandler.CreateBulk)
		auth.DELETE("/lessons", lessonHandler.DeleteByCourse)
		auth.GET("/lessons/:id", lessonHandler.GetByID)
		auth.PUT("/lessons/:id", lessonHandler.Update)
		auth.DELETE("/lessons/:id", lessonHandler.Delete)
		auth.DELETE("/lessons/series/:seriesId", lessonHandler.DeleteSeries)
		auth.PATCH("/lessons/series/:seriesId", lessonHandler.UpdateSeries)

		auth.GET("/calendar", lessonHandler.GetCalendar)
		auth.GET("/dashboard/cycles", lessonHandler.GetCurrentCycles)

		auth.GET("/courses/:id/enrollments", enrollmentHandler.GetByCourse)
		auth.POST("/courses/:id/enrollments", enrollmentHandler.Add)
		auth.DELETE("/courses/:id/enrollments/:studentId", enrollmentHandler.Remove)

		auth.GET("/lessons/:id/attendance", attendanceHandler.Get)
		auth.PUT("/lessons/:id/attendance", attendanceHandler.Update)

		auth.GET("/tasks", taskHandler.GetByRange)
		auth.GET("/tasks/board", taskHandler.GetBoard)
		auth.POST("/tasks", taskHandler.Create)
		auth.PUT("/tasks/:id", taskHandler.Update)
		auth.DELETE("/tasks/:id", taskHandler.Delete)

		auth.POST("/lessons/:id/room-token", callHandler.GetToken)
		auth.POST("/lessons/:id/start-room", callHandler.StartRoom)
		auth.POST("/lessons/:id/end-room", callHandler.EndRoom)
		auth.POST("/calls/quick", callHandler.StartQuickRoom)
		auth.POST("/calls/quick/:id/end", callHandler.EndQuickRoom)

		// Whiteboard protected routes
		auth.GET("/boards/trial", whiteboardHandler.GetTrialBoard)
		auth.GET("/boards/course/:courseId", whiteboardHandler.GetBoardByCourse)
		auth.POST("/boards/:boardId/pages", whiteboardHandler.CreatePage)
		auth.PUT("/board-pages/:pageId", whiteboardHandler.UpdatePage)
		auth.DELETE("/board-pages/:pageId", whiteboardHandler.DeletePage)
		auth.POST("/boards/:boardId/invite", whiteboardHandler.CreateInvite)
		auth.DELETE("/boards/:boardId/invite", whiteboardHandler.DeleteInvite)
		auth.POST("/boards/:boardId/assets", whiteboardHandler.UploadAsset)

		// Materials library
		auth.GET("/materials", materialHandler.List)
		auth.POST("/materials/folder", materialHandler.CreateFolder)
		auth.POST("/materials", materialHandler.Upload)
		auth.DELETE("/materials/:id", materialHandler.Delete)
		auth.GET("/materials/:id/url", materialHandler.GetURL)
	}

	// Protected student routes (student JWT role, no subscription check — that's the tutor's concern)
	stu := r.Group("/student")
	stu.Use(middleware.AuthStudent(cfg.JWTSecret))
	{
		stu.POST("/lessons/:id/room-token", callHandler.GetStudentToken)
		stu.GET("/lessons/:id/board-token", whiteboardHandler.StudentBoardToken)
		stu.GET("/me", studentHandler.Me)
		stu.GET("/lessons", studentHandler.ListLessons)
		stu.GET("/homework", studentHandler.Homework)
		stu.GET("/courses", studentHandler.Courses)
		stu.GET("/courses/:id/board-token", whiteboardHandler.StudentCourseBoardToken)
		stu.POST("/password", studentAuthHandler.ChangePassword)
	}

	return r, subscriptionService
}
