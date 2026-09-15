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
	"tutorgo/jobs"
	"tutorgo/middleware"
	"tutorgo/repository"
	"tutorgo/service"
	"tutorgo/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"golang.org/x/time/rate"
)

func Setup(pool *pgxpool.Pool, log *slog.Logger, cfg *config.Config) (*gin.Engine, service.SubscriptionService, *handlers.WbHubManager) {
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
	eventRepo := repository.NewEventRepository(pool)
	whiteboardRepo := repository.NewWhiteboardRepository(pool)
	subscriptionRepo := repository.NewSubscriptionRepository(pool)
	studentRefreshRepo := repository.NewStudentRefreshTokenRepository(pool)
	pendingRepo := repository.NewPendingRegistrationRepository(pool)
	materialRepo := repository.NewMaterialRepository(pool)
	pdfImportRepo := repository.NewPdfImportRepository(pool)

	// Services
	tutorService := service.NewTutorService(tutorRepo, subscriptionRepo, pool)
	emailSender := email.NewSender(cfg.ResendAPIKey, cfg.EmailFrom, log)
	registrationService := service.NewRegistrationService(pendingRepo, tutorService, emailSender, cfg.AppURL, log)
	refreshTokenService := service.NewRefreshTokenService(refreshTokenRepo)
	studentRefreshService := service.NewStudentRefreshTokenService(studentRefreshRepo)
	paymentService := service.NewPaymentService(paymentRepo, courseRepo, enrollmentRepo)
	recurrenceService := service.NewRecurrenceService(repository.NewRecurrenceRepository(pool))
	lessonService := service.NewLessonService(lessonRepo, courseRepo, paymentRepo, recurrenceService)
	// Ниже lessonService: архивация курса ходит к нему за закрытием серий.
	// Цикла нет — lessonService зависит от courseRepo, а не от courseService.
	courseService := service.NewCourseService(courseRepo, studentRepo, lessonService)
	// Ниже courseService: архивация ученика архивирует его курсы именно сервисом —
	// у courseRepo тот же Delete, но без закрытия правил и будущих уроков.
	studentService := service.NewStudentService(studentRepo, paymentRepo, courseService, enrollmentRepo, studentRefreshRepo)
	enrollmentService := service.NewEnrollmentService(enrollmentRepo, courseRepo, studentRepo)
	attendanceService := service.NewAttendanceService(attendanceRepo, lessonRepo, courseRepo)
	taskService := service.NewTaskService(taskRepo)
	eventService := service.NewEventService(eventRepo, recurrenceService)
	calendarService := service.NewCalendarService(lessonService, eventRepo, taskRepo)
	onboardingService := service.NewOnboardingService(studentService, courseService, lessonService)
	icsService := service.NewICSService(calendarService, tutorRepo)
	whiteboardService := service.NewWhiteboardService(whiteboardRepo)
	subscriptionService := service.NewSubscriptionService(subscriptionRepo, service.StubProvider{})
	materialService := service.NewMaterialService(materialRepo)

	// Insert-only River-клиент: воркеров в API-роли нет, только постановка джоб.
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		log.Error("river client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	pdfImportService := service.NewPdfImportService(pdfImportRepo,
		func(ctx context.Context, tx pgx.Tx, importID string) error {
			_, err := riverClient.InsertTx(ctx, tx, jobs.PdfImportArgs{ImportID: importID}, nil)
			return err
		})

	// Handlers
	tutorHandler := handlers.NewTutorHandler(tutorService, refreshTokenService, log)
	authHandler := handlers.NewAuthHandler(tutorService, registrationService, refreshTokenService, log, cfg.JWTSecret, cfg.Env == "production")
	studentHandler := handlers.NewStudentHandler(studentService, log)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService, log)
	icsHandler := handlers.NewICSHandler(icsService, log)
	courseHandler := handlers.NewCourseHandler(courseService, log)
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)
	lessonHandler := handlers.NewLessonHandler(lessonService, log)
	enrollmentHandler := handlers.NewEnrollmentHandler(enrollmentService, log)
	attendanceHandler := handlers.NewAttendanceHandler(attendanceService, log)
	taskHandler := handlers.NewTaskHandler(taskService, log)
	eventHandler := handlers.NewEventHandler(eventService, log)
	calendarHandler := handlers.NewCalendarHandler(calendarService, log)
	callHandler := handlers.NewCallHandler(lessonService, log, cfg.LiveKitURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret, studentService, tutorService)
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
	pdfImportHandler := handlers.NewPdfImportHandler(pdfImportService, store, log)

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
		// Снапшот доски — единственное легитимно крупное JSON-тело: сцена с
		// импортированным PDF и рукописными пометками не влезает в 1 МБ.
		if c.FullPath() == "/public/board/pages/:pageId/snapshot" {
			limit = 16 << 20
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
	// Логин/refresh/logout делят одно ведро на IP: 10 попыток подряд, потом 1 раз в 3 с.
	// Человек, вспоминающий пароль, сюда не упирается; сюда же ложится фоновый refresh.
	authLimiter := middleware.RateLimit(rate.Every(3*time.Second), 15)
	// Регистрация/resend шлют письма — жёстче лимит по IP (≈5/час), беречь sender-репутацию.
	// У resend поверх этого ещё per-email кулдаун 60 с в service/registration.go.
	regLimiter := middleware.RateLimit(rate.Every(12*time.Minute), 5)
	r.POST("/auth/register", regLimiter, authHandler.Register)
	r.POST("/auth/register/verify", authLimiter, authHandler.RegisterVerify)
	r.POST("/auth/register/resend", regLimiter, authHandler.RegisterResend)
	r.POST("/auth/login", authLimiter, authHandler.Login)
	r.POST("/auth/refresh", authLimiter, authHandler.Refresh)
	r.POST("/auth/logout", authLimiter, authHandler.Logout)
	r.GET("/public/lessons/:id/room-status", callHandler.GetRoomStatus)
	// Статус гость поллит раз в 5 с, пока ждёт начала урока, и каждый вызов теперь
	// идёт в LiveKit — поэтому лимит есть, но мягче гостевого токена: несколько
	// ожидающих за одним NAT не должны глушить друг друга.
	r.GET("/public/quick/:id/status", middleware.RateLimit(rate.Every(time.Second), 10), callHandler.GetQuickRoomStatus)
	r.GET("/public/quick/:id/guest-token", middleware.RateLimit(rate.Every(3*time.Second), 5), callHandler.GetQuickGuestToken)
	// Публичная по назначению: сюда ходит Google Календарь без авторизации,
	// секрет — сама ссылка. Rate-limit держит перебор токенов в рамках.
	r.GET("/ics/:token", middleware.RateLimit(rate.Every(time.Second), 10), icsHandler.Feed)
	r.POST("/webhooks/livekit", callHandler.LiveKitWebhook)
	r.POST("/subscription/webhook", subscriptionHandler.Webhook)

	// Public student auth routes
	studentAuthLimiter := middleware.RateLimit(rate.Every(3*time.Second), 15)
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
	// Персист доски: публичен как и WS (гость-ученик правит доску по invite),
	// авторизуется тем же ?token=. См. WhiteboardHandler.SaveSnapshot.
	r.POST("/public/board/pages/:pageId/snapshot", whiteboardHandler.SaveSnapshot)

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
		// Ученик + курс + серия одним сабмитом: обычный /students остаётся для
		// правки карточки, этот — для первого шага.
		auth.POST("/onboarding/student", onboardingHandler.CreateStudent)
		auth.POST("/ics/link", icsHandler.EnsureLink)
		auth.DELETE("/ics/link", icsHandler.RevokeLink)
		auth.GET("/students/:id", studentHandler.GetByID)
		auth.PUT("/students/:id", studentHandler.Update)
		auth.DELETE("/students/:id", studentHandler.Delete)
		auth.POST("/students/:id/archive", studentHandler.Archive)
		auth.POST("/students/:id/restore", studentHandler.Restore)
		auth.GET("/students/:id/courses", courseHandler.GetByStudent)
		auth.POST("/students/:id/invite", studentHandler.Invite)

		auth.GET("/courses", courseHandler.GetAll)
		auth.POST("/courses", courseHandler.Create)
		auth.GET("/courses/archived", courseHandler.GetArchived)
		auth.GET("/courses/subjects", courseHandler.GetSubjects)
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
		auth.GET("/payments/debts", paymentHandler.GetDebts)
		auth.GET("/payments/monthly-income", paymentHandler.GetMonthlyIncome)
		auth.GET("/payments/monthly-expected", paymentHandler.GetMonthlyExpected)

		auth.GET("/lessons", lessonHandler.GetByCourse)
		auth.POST("/lessons", lessonHandler.Create)
		auth.DELETE("/lessons", lessonHandler.DeleteByCourse)
		auth.GET("/lessons/:id", lessonHandler.GetByID)
		auth.PUT("/lessons/:id", lessonHandler.Update)
		auth.DELETE("/lessons/:id", lessonHandler.Delete)

		// /calendar — только уроки (дашборд, сайдбар), /calendar/feed — единая лента.
		auth.GET("/calendar", lessonHandler.GetCalendar)
		auth.GET("/calendar/feed", calendarHandler.GetFeed)
		auth.GET("/calendar/conflicts", calendarHandler.GetConflicts)
		auth.GET("/dashboard/cycles", lessonHandler.GetCurrentCycles)

		auth.GET("/events", eventHandler.GetByRange)
		auth.POST("/events", eventHandler.Create)
		auth.GET("/events/:id", eventHandler.GetByID)
		auth.PUT("/events/:id", eventHandler.Update)
		auth.DELETE("/events/:id", eventHandler.Delete)

		auth.GET("/courses/:id/enrollments", enrollmentHandler.GetByCourse)
		auth.POST("/courses/:id/enrollments", enrollmentHandler.Add)
		auth.POST("/courses/:id/enrollments/bulk", enrollmentHandler.AddBulk)
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
		auth.POST("/boards/:boardId/pdf", pdfImportHandler.Upload)
		auth.POST("/pdf-imports/:id/start", pdfImportHandler.Start)

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

	return r, subscriptionService, wbHubManager
}
