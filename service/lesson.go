package service

import (
	"context"
	"fmt"
	"tutorgo/models"
	"tutorgo/repository"
)

type LessonService interface {
	Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error)
	CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest, tutorID string) ([]models.Lesson, error)
	GetByCourse(ctx context.Context, courseID string, tutorID string) ([]models.Lesson, error)
	GetByCoursePaged(ctx context.Context, courseID string, tutorID string, p models.Pagination) (models.PagedResponse[models.Lesson], error)
	GetByID(ctx context.Context, id string, tutorID string) (models.Lesson, error)
	Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error)
	Delete(ctx context.Context, id string, tutorID string) error
	DeleteByCourse(ctx context.Context, courseID string, tutorID string) error
	DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error
	UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error
	GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error)
	GetByPeriod(ctx context.Context, courseID string, tutorID string, from string, to string) ([]models.Lesson, error)
	ExistsPublic(ctx context.Context, id string) error
	StartRoom(ctx context.Context, lessonID string, tutorID string) error
	EndRoom(ctx context.Context, lessonID string, tutorID string) error
	GetRoomStatus(ctx context.Context, id string) (string, error)
}

type lessonService struct {
	repo        repository.LessonRepository
	courseRepo  repository.CourseRepository
	paymentRepo repository.PaymentRepository
}

func NewLessonService(repo repository.LessonRepository, courseRepo repository.CourseRepository, paymentRepo repository.PaymentRepository) LessonService {
	return &lessonService{repo: repo, courseRepo: courseRepo, paymentRepo: paymentRepo}
}

func (s *lessonService) enrichCalendarLessons(ctx context.Context, lessons []models.CalendarLesson) error {
	if len(lessons) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	courseIDs := make([]string, 0)
	for _, l := range lessons {
		if !seen[l.CourseID] {
			seen[l.CourseID] = true
			courseIDs = append(courseIDs, l.CourseID)
		}
	}
	ranks, err := s.repo.GetRanksForCourses(ctx, courseIDs)
	if err != nil {
		return err
	}
	paymentsMap, err := s.paymentRepo.GetByCoursesBatch(ctx, courseIDs)
	if err != nil {
		return err
	}
	cycleInfosByCourse := make(map[string]map[string]cycleInfo)
	for _, courseID := range courseIDs {
		coursePayments := paymentsMap[courseID]
		if len(coursePayments) == 0 {
			continue
		}
		cycleInfosByCourse[courseID] = computeCyclePositions(ranks[courseID], coursePayments)
	}
	for i, l := range lessons {
		infos := cycleInfosByCourse[l.CourseID]
		if infos == nil {
			continue
		}
		if info, ok := infos[l.ID]; ok {
			pos, size := info.Position, info.Size
			lessons[i].CyclePosition = &pos
			lessons[i].CycleSize = &size
		}
	}
	return nil
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

func (s *lessonService) Create(ctx context.Context, req models.CreateLessonRequest, tutorID string) (models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.Create(ctx, req)
}

func (s *lessonService) CreateBulk(ctx context.Context, req models.CreateBulkLessonRequest, tutorID string) ([]models.Lesson, error) {
	_, err := s.courseRepo.GetByID(ctx, req.CourseID, tutorID)
	if err != nil {
		return nil, fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.CreateBulk(ctx, req)
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

func (s *lessonService) Update(ctx context.Context, id string, req models.UpdateLessonRequest, tutorID string) (models.Lesson, error) {
	_, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return models.Lesson{}, fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return s.repo.Update(ctx, id, req)
}

func (s *lessonService) Delete(ctx context.Context, id string, tutorID string) error {
	_, err := s.repo.GetByIDForTutor(ctx, id, tutorID)
	if err != nil {
		return fmt.Errorf("lesson: %w", ErrNotFound)
	}
	return s.repo.Delete(ctx, id)
}

func (s *lessonService) DeleteByCourse(ctx context.Context, courseID string, tutorID string) error {
	_, err := s.courseRepo.GetByID(ctx, courseID, tutorID)
	if err != nil {
		return fmt.Errorf("course: %w", ErrNotFound)
	}
	return s.repo.DeleteByCourse(ctx, courseID, tutorID)
}

func (s *lessonService) DeleteSeries(ctx context.Context, seriesID string, tutorID string, fromDate *string) error {
	return s.repo.DeleteSeries(ctx, seriesID, tutorID, fromDate)
}

func (s *lessonService) UpdateSeries(ctx context.Context, seriesID string, tutorID string, req models.UpdateSeriesRequest) error {
	if req.NewTime == nil && req.DurationMinutes == nil && req.Notes == nil {
		return fmt.Errorf("update requires at least one field: %w", ErrBadRequest)
	}
	return s.repo.UpdateSeries(ctx, seriesID, tutorID, req)
}

func (s *lessonService) GetCalendar(ctx context.Context, tutorID string, from string, to string) ([]models.CalendarLesson, error) {
	lessons, err := s.repo.GetCalendar(ctx, tutorID, from, to)
	if err != nil {
		return nil, err
	}
	if err := s.enrichCalendarLessons(ctx, lessons); err != nil {
		return nil, err
	}
	return lessons, nil
}

func (s *lessonService) ExistsPublic(ctx context.Context, id string) error {
	return s.repo.ExistsPublic(ctx, id)
}

func (s *lessonService) StartRoom(ctx context.Context, lessonID string, tutorID string) error {
	return s.repo.StartRoom(ctx, lessonID, tutorID)
}

func (s *lessonService) EndRoom(ctx context.Context, lessonID string, tutorID string) error {
	return s.repo.EndRoom(ctx, lessonID, tutorID)
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
