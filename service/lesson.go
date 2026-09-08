package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
	"tutorgo/models"
	"tutorgo/repository"
)

type LessonService interface {
	Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error)
	GetByCoursePaged(ctx context.Context, courseID string, tutorID string, p models.Pagination) (models.PagedResponse[models.Lesson], error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID, scope string) (models.Lesson, error)
	Delete(ctx context.Context, id string, tutorID, scope string) error
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	GetCurrentCycles(ctx context.Context, tutorID string) ([]models.CurrentCycleInfo, error)
	GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error)
	StartRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoomByID(ctx context.Context, lessonID string) error
	GetRoomStatus(ctx context.Context, id string) (string, error)
}

type lessonService struct {
	repo        repository.LessonRepository
	courseRepo  repository.CourseRepository
	paymentRepo repository.PaymentRepository
	recurrence  RecurrenceService
}

func NewLessonService(
	repo repository.LessonRepository,
	courseRepo repository.CourseRepository,
	paymentRepo repository.PaymentRepository,
	recurrence RecurrenceService,
) LessonService {
	return &lessonService{repo: repo, courseRepo: courseRepo, paymentRepo: paymentRepo, recurrence: recurrence}
}

// cyclePositionFromRank computes a lesson's position and size within its payment cycle.
// rank is the lesson's 1-based global position among non-cancelled lessons in the course.
func cyclePositionFromRank(rank int, payments []models.Payment) (position, size int) {
	cum := 0
	for _, p := range payments {
		cum += p.LessonsCount
		if rank <= cum {
			return rank - (cum - p.LessonsCount), p.LessonsCount
		}
	}
	return 0, 0
}

func (s *lessonService) enrichLessons(ctx context.Context, courseID string, lessons []models.Lesson) error {
	if len(lessons) == 0 {
		return nil
	}
	ranks, err := s.repo.GetRanksForCourses(ctx, []string{courseID})
	if err != nil {
		return err
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, []string{courseID})
	if err != nil {
		return err
	}
	coursePayments := paymentsMap[courseID]
	if len(coursePayments) == 0 {
		return nil
	}
	infos := computeCyclePositions(ranks[courseID], coursePayments)
	for i, l := range lessons {
		if info, ok := infos[l.ID]; ok {
			pos, size := info.Position, info.Size
			lessons[i].CyclePosition = &pos
			lessons[i].CycleSize = &size
		}
	}
	return nil
}

// resolveCourse отдаёт курс, в который ляжет урок: либо существующий по
// course_id, либо найденный/созданный по паре «ученик + предмет». Второй путь —
// постановка урока из календаря, где пользователь про курсы не думает вовсе.
func (s *lessonService) resolveCourse(ctx context.Context, tutorID, courseID, studentID, subject string, startedAt time.Time) (models.Course, error) {
	if courseID == "" {
		if studentID == "" || subject == "" {
			return models.Course{}, fmt.Errorf("course_id or student_id with subject required: %w", ErrBadRequest)
		}
		// Курс создаётся уже активным, проверять IsActive нечего.
		course, err := s.courseRepo.GetOrCreateIndividual(ctx, tutorID, studentID, subject, startedAt)
		if err != nil {
			return models.Course{}, fmt.Errorf("student: %w", ErrNotFound)
		}
		return course, nil
	}

	course, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return models.Course{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	if !course.IsActive {
		return models.Course{}, fmt.Errorf("course is archived: %w", ErrConflict)
	}
	return course, nil
}

// firstScheduledAt — дата первого урока серии, она же started_at неявного курса.
// Мусор в строке разберёт репозиторий при вставке; курсу хватит сегодняшней даты.
func firstScheduledAt(ats []string) time.Time {
	if len(ats) == 0 {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, ats[0])
	if err != nil {
		return time.Now()
	}
	return t
}

func (s *lessonService) Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	course, err := s.resolveCourse(ctx, tutorID, req.CourseID, req.StudentID, req.Subject, req.ScheduledAt)
	if err != nil {
		return models.Lesson{}, err
	}
	req.CourseID = course.ID

	if req.Recurrence != nil {
		return s.createSeries(ctx, req, tutorID)
	}

	lesson, err := s.repo.Create(ctx, req)
	if err == nil {
		globalCalendarCache.Invalidate(tutorID)
	}
	return lesson, err
}

// createSeries заводит правило, создаёт по нему первый урок — он же шаблон для
// материализации — и добивает горизонт. Возвращается именно первый урок:
// клиенту нужен он, остальные серия отдаст через календарь.
func (s *lessonService) createSeries(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	rule, err := s.recurrence.CreateRule(ctx, *req.Recurrence, req.ScheduledAt, req.DurationMinutes, tutorID)
	if err != nil {
		return models.Lesson{}, err
	}

	req.RuleID = rule.ID
	req.OccurrenceDate = &rule.StartsOn

	lesson, err := s.repo.Create(ctx, req)
	if err != nil {
		// Правило без первого вхождения — сирота: шаблона для материализации у
		// него нет, зато ночная джоба будет ходить к нему каждый день.
		if delErr := s.recurrence.DeleteRule(ctx, rule.ID); delErr != nil {
			return models.Lesson{}, fmt.Errorf("create lesson: %w (orphan rule %s)", err, rule.ID)
		}
		return models.Lesson{}, err
	}

	// Ошибка материализации не отменяет уже созданный урок: ночная джоба
	// догонит горизонт, а пользователь получит хотя бы первое занятие.
	if _, err := s.recurrence.Materialize(ctx, rule.ID, time.Now().Add(RecurrenceHorizon)); err != nil {
		return lesson, nil
	}
	globalCalendarCache.Invalidate(tutorID)
	return lesson, nil
}

func (s *lessonService) GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.GetByCourse(ctx, courseID)
}

func (s *lessonService) GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	lessons, err := s.repo.GetByPeriod(ctx, courseID, tutorID, from, to)
	if err != nil {
		return nil, err
	}
	if err := s.enrichLessons(ctx, courseID, lessons); err != nil {
		return nil, err
	}
	return lessons, nil
}

func (s *lessonService) GetByCoursePaged(ctx context.Context, courseID string, tutorID string, p models.Pagination) (models.PagedResponse[models.Lesson], error) {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return models.PagedResponse[models.Lesson]{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	lessons, total, err := s.repo.GetByCoursePaged(ctx, courseID, p)
	if err != nil {
		return models.PagedResponse[models.Lesson]{}, err
	}
	return models.PagedResponse[models.Lesson]{
		Data:  lessons,
		Total: total,
		Page:  p.Page,
		Limit: p.Limit,
	}, nil
}

func (s *lessonService) GetByID(ctx context.Context, id string, tutorID string) (models.Lesson, error) {
	lesson, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return lesson, nil
}

// Update правит урок в одной из трёх областей. Для урока вне серии scope не
// значит ничего: правится он один.
//
// scope = one       — только это вхождение (перенос мышью всегда такой);
//         following — это и все следующие: правило разрезается на две ветки;
//         all       — всё правило целиком, прошедшие вхождения не трогаются.
func (s *lessonService) Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID, scope string) (models.Lesson, error) {
	if !validScope(scope) {
		return models.Lesson{}, fmt.Errorf("scope %q: %w", scope, ErrBadRequest)
	}
	lesson, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("lesson: %w", ErrNotFound)
	}

	if scope == scopeOne || lesson.RuleID == nil || lesson.OccurrenceDate == nil {
		updated, err := s.repo.Update(ctx, id, req)
		if err != nil {
			return updated, err
		}
		// Вхождение, ушедшее со своего места в расписании, метим: иначе
		// следующее «это и все следующие» снесёт его и материализация вернёт
		// урок обратно. Статус и заметки правилом не задаются — они с
		// расписанием не спорят и метки не заслуживают.
		movedOffSchedule := !req.ScheduledAt.Equal(lesson.ScheduledAt) ||
			req.DurationMinutes != lesson.DurationMinutes
		if lesson.RuleID != nil && movedOffSchedule {
			if err := s.repo.MarkOverride(ctx, id); err != nil {
				return updated, err
			}
		}
		globalCalendarCache.Invalidate(tutorID)
		return updated, nil
	}
	return s.updateSeries(ctx, lesson, req, tutorID, scope)
}

func (s *lessonService) updateSeries(ctx context.Context, lesson models.Lesson, req models.UpdateLessonRequest, tutorID, scope string) (models.Lesson, error) {
	rule, err := s.recurrence.GetRule(ctx, *lesson.RuleID)
	if err != nil {
		return models.Lesson{}, err
	}
	// Серию задают день недели и время; выбранная в форме неделя роли не играет.
	// Без нормализации строка урока и occurrence_date разъедутся.
	req.ScheduledAt, err = NormalizeSeriesStart(rule, *lesson.OccurrenceDate, req.ScheduledAt)
	if err != nil {
		return models.Lesson{}, err
	}

	// Своя строка идёт первой: если она прошла, а Retime упал, урок стоит на
	// новом времени при нетронутой серии — состояние видимое и чинится повтором.
	updated, err := s.repo.Update(ctx, lesson.ID, req)
	if err != nil {
		return models.Lesson{}, err
	}
	// defer сразу после успешного repo.Update: строка уже подвинута, и кеш
	// обязан протухнуть даже если Retime дальше упадёт с ошибкой (в т.ч.
	// ErrConflict из пре-флайта занятой даты) — иначе календарь молчит про
	// изменение, которое всё-таки произошло.
	defer globalCalendarCache.Invalidate(tutorID)
	if err := s.recurrence.Retime(ctx, rule, lesson.ID, *lesson.OccurrenceDate,
		req.ScheduledAt, req.DurationMinutes, scope); err != nil {
		return models.Lesson{}, err
	}
	return updated, nil
}

func (s *lessonService) Delete(ctx context.Context, id string, tutorID, scope string) error {
	if !validScope(scope) {
		return fmt.Errorf("scope %q: %w", scope, ErrBadRequest)
	}
	lesson, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("lesson: %w", ErrNotFound)
	}
	defer globalCalendarCache.Invalidate(tutorID)

	if lesson.RuleID == nil || lesson.OccurrenceDate == nil {
		return s.repo.Delete(ctx, id)
	}

	switch scope {
	case scopeFollowing:
		if err := s.recurrence.PruneFuture(ctx, *lesson.RuleID, *lesson.OccurrenceDate); err != nil {
			return err
		}
		if err := s.recurrence.CloseRule(ctx, *lesson.RuleID, *lesson.OccurrenceDate); err != nil {
			return err
		}
		return s.repo.Cancel(ctx, id)

	case scopeAll:
		// Порядок важен: удаление правила обнуляет rule_id у уроков
		// (ON DELETE SET NULL), и найти вхождения станет нечем.
		if err := s.recurrence.PruneFuture(ctx, *lesson.RuleID, time.Time{}); err != nil {
			return err
		}
		return s.recurrence.DeleteRule(ctx, *lesson.RuleID)

	default:
		// Отменяем, а не удаляем: свободная дата вернула бы урок обратно на
		// ближайшей материализации.
		return s.repo.Cancel(ctx, id)
	}
}

// ArchiveCourseSchedule закрывает серии курса и убирает будущие уроки.
// Завершённые остаются — так обещает диалог архивации, и на них висят
// посещаемость, платежи и доски.
//
// Правило закрываем, а не удаляем: у прошедших уроков rule_id остаётся, и
// история занятий сохраняет признак серии. Из DueForMaterialization закрытое
// правило выпадает по ends_on > CURRENT_DATE.
//
// Порядок безопасен и при обрыве посередине: закрытие правил идёт первым и
// сразу останавливает материализацию, поэтому оборванный вызов оставляет
// видимое состояние «правила закрыты, устаревшие будущие уроки на месте» —
// лечится повторной архивацией, тихой порчи нет.
func (s *lessonService) ArchiveCourseSchedule(ctx context.Context, courseID, tutorID string) error {
	ruleIDs, err := s.repo.GetRuleIDsByCourse(ctx, courseID)
	if err != nil {
		return err
	}
	for _, id := range ruleIDs {
		// CloseRule ставит ends_on = at − 1, то есть вчера: всё, что позже,
		// серии больше не принадлежит.
		if err := s.recurrence.CloseRule(ctx, id, time.Now()); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteFutureByCourse(ctx, courseID, tutorID); err != nil {
		return err
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}

func (s *lessonService) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	if _, err := s.courseRepo.GetByID(ctx, courseID, tutorID); err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	// Связь правила с курсом — только через lessons.rule_id: читаем до удаления.
	ruleIDs, err := s.repo.GetRuleIDsByCourse(ctx, courseID)
	if err != nil {
		return err
	}
	// Правила удаляем ДО уроков: lessons.rule_id — ON DELETE SET NULL
	// (migrations/033_recurrence.sql), поэтому удаление правила лишь обнулит
	// rule_id у его уроков, которых через строку всё равно снесёт DeleteByCourse.
	// Обрыв посередине тогда не оставляет сироту: без правил уроки-сироты
	// структурно невозможны, а сам обрыв виден как незавершённое удаление
	// (уроки на месте) и лечится повтором.
	for _, id := range ruleIDs {
		if err := s.recurrence.DeleteRule(ctx, id); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteByCourse(ctx, courseID, tutorID); err != nil {
		return err
	}
	globalCalendarCache.Invalidate(tutorID)
	return nil
}

func (s *lessonService) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	if cached, ok := globalCalendarCache.get(tutorID, from, to); ok {
		return cached, nil
	}

	var (
		lessons     []models.CalendarLesson
		paymentsMap map[string][]models.Payment
		lessonsErr  error
		paymentsErr error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		lessons, lessonsErr = s.repo.GetCalendar(ctx, tutorID, from, to)
	}()
	go func() {
		defer wg.Done()
		paymentsMap, paymentsErr = s.paymentRepo.GetPaymentsForCalendar(ctx, tutorID, from, to)
	}()
	wg.Wait()

	if lessonsErr != nil {
		return nil, lessonsErr
	}
	if paymentsErr != nil {
		return nil, paymentsErr
	}

	for i, l := range lessons {
		if l.Rank == nil {
			continue
		}
		coursePayments := paymentsMap[l.CourseID]
		if len(coursePayments) == 0 {
			continue
		}
		pos, size := cyclePositionFromRank(*l.Rank, coursePayments)
		if pos > 0 {
			p, sz := pos, size
			lessons[i].CyclePosition = &p
			lessons[i].CycleSize = &sz
		}
	}

	globalCalendarCache.set(tutorID, from, to, lessons)
	return lessons, nil
}

func (s *lessonService) GetCurrentCycles(ctx context.Context, tutorID string) ([]models.CurrentCycleInfo, error) {
	lessons, err := s.repo.GetAllLessonsForCycles(ctx, tutorID)
	if err != nil {
		return nil, err
	}
	if len(lessons) == 0 {
		return nil, nil
	}

	type lessonMeta struct {
		scheduledAt time.Time
		status      string
		rank        int
	}
	type courseMeta struct {
		subject     string
		studentName *string
		lessons     []lessonMeta
	}

	coursesByID := map[string]*courseMeta{}
	for _, l := range lessons {
		if l.Rank == nil {
			continue
		}
		if _, ok := coursesByID[l.CourseID]; !ok {
			coursesByID[l.CourseID] = &courseMeta{subject: l.Subject, studentName: l.StudentName}
		}
		coursesByID[l.CourseID].lessons = append(coursesByID[l.CourseID].lessons, lessonMeta{
			scheduledAt: l.ScheduledAt, status: l.Status, rank: *l.Rank,
		})
	}

	courseIDs := make([]string, 0, len(coursesByID))
	for id := range coursesByID {
		courseIDs = append(courseIDs, id)
	}

	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, courseIDs)
	if err != nil {
		return nil, err
	}

	var result []models.CurrentCycleInfo
	for courseID, meta := range coursesByID {
		payments := paymentsMap[courseID]
		if len(payments) == 0 {
			continue
		}

		// payments are ordered by paid_at ASC per GetByCoursesBatch
		bounds := make([]int, len(payments))
		cum := 0
		for i, p := range payments {
			cum += p.LessonsCount
			bounds[i] = cum
		}

		buckets := make([][]lessonMeta, len(payments))
		for _, lm := range meta.lessons {
			for i, bound := range bounds {
				prev := 0
				if i > 0 {
					prev = bounds[i-1]
				}
				if lm.rank > prev && lm.rank <= bound {
					buckets[i] = append(buckets[i], lm)
					break
				}
			}
		}

		currentIdx := -1
		for i := len(buckets) - 1; i >= 0; i-- {
			for _, lm := range buckets[i] {
				if lm.status == "scheduled" {
					currentIdx = i
					break
				}
			}
			if currentIdx >= 0 {
				break
			}
		}
		if currentIdx < 0 {
			continue
		}

		bucket := buckets[currentIdx]
		cycleSize := payments[currentIdx].LessonsCount
		var lastAt time.Time
		progress := 0
		for _, lm := range bucket {
			if lm.status == "completed" || lm.status == "missed" {
				progress++
			}
			if lm.scheduledAt.After(lastAt) {
				lastAt = lm.scheduledAt
			}
		}

		result = append(result, models.CurrentCycleInfo{
			CourseID:    courseID,
			Subject:     meta.subject,
			StudentName: meta.studentName,
			Progress:    progress,
			CycleSize:   cycleSize,
			LastAt:      lastAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].LastAt.After(result[j].LastAt)
	})
	return result, nil
}

func (s *lessonService) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	return s.repo.StartRoom(ctx, lessonID, tutorID)
}

func (s *lessonService) EndRoom(ctx context.Context, lessonID string, tutorID string) error {
	return s.repo.EndRoom(ctx, lessonID, tutorID)
}

func (s *lessonService) EndRoomByID(ctx context.Context, lessonID string) error {
	return s.repo.EndRoomByID(ctx, lessonID)
}

func (s *lessonService) GetRoomStatus(ctx context.Context, id string) (string, error) {
	return s.repo.GetRoomStatus(ctx, id)
}

type cycleInfo struct {
	Position int
	Size     int
}

func computeCyclePositions(ranks map[string]int, payments []models.Payment) map[string]cycleInfo {
	if len(payments) == 0 {
		return nil
	}

	bounds := make([]int, len(payments))
	cum := 0
	for i, p := range payments {
		cum += p.LessonsCount
		bounds[i] = cum
	}

	result := make(map[string]cycleInfo)
	for lessonID, rank := range ranks {
		for i, bound := range bounds {
			prev := 0
			if i > 0 {
				prev = bounds[i-1]
			}
			if rank > prev && rank <= bound {
				result[lessonID] = cycleInfo{
					Position: rank - prev,
					Size:     payments[i].LessonsCount,
				}
				break
			}
		}
	}
	return result
}
