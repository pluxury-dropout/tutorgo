//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Платёж адресный (спека 2026-09-06-price-units…, п. 6.1–6.3). Запуск:
// make test-integration.

// addStudent — ещё один ученик репетитора.
func addStudent(t *testing.T, pool *pgxpool.Pool, tutorID, firstName string) string {
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO students (tutor_id, first_name) VALUES ($1, $2) RETURNING id`,
		tutorID, firstName).Scan(&id))
	return id
}

// enroll — запись в группу с явной датой записи; atExpr — SQL-выражение.
func enroll(t *testing.T, pool *pgxpool.Pool, courseID, studentID, atExpr string) {
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO course_enrollments (course_id, student_id, enrolled_at)
		             VALUES ($1, $2, %s)`, atExpr),
		courseID, studentID)
	require.NoError(t, err)
}

// payFor — адресный платёж ученика по курсу за lessons уроков.
func payFor(t *testing.T, pool *pgxpool.Pool, courseID, studentID string, lessons int) {
	_, err := pool.Exec(context.Background(),
		`INSERT INTO payments (course_id, student_id, amount, lessons_count, paid_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		courseID, studentID, float64(lessons*1000), lessons)
	require.NoError(t, err)
}

// Ушедшему из группы платёж обязан приниматься — иначе долг невзыскиваемый;
// ученику, которого в группе не было, — нет (спека, п. 6.3).
func TestIsEnrolled_TrueForLeftStudentFalseForStranger(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, studentID := seedTutorStudent(t, pool)
	stranger := addStudent(t, pool, tutorID, "Чужой")
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, studentID, "NOW() - interval '1 month'")

	repo := repository.NewEnrollmentRepository(pool)
	require.NoError(t, repo.Remove(ctx, groupID, studentID))

	ok, err := repo.IsEnrolled(ctx, groupID, studentID)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = repo.IsEnrolled(ctx, groupID, stranger)
	require.NoError(t, err)
	assert.False(t, ok)
}

// Участник группы, заплативший, но ни разу не отмеченный, — ученик с историей:
// иначе каскад по payments.student_id унёс бы его платёж (спека, п. 6.3).
func TestStudentDelete_AddressedGroupPaymentBlocksDeletion(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, studentID, "NOW()")
	payFor(t, pool, groupID, studentID, 4)

	deleted, err := repository.NewStudentRepository(pool).Delete(context.Background(), studentID, tutorID)
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.True(t, studentExists(t, pool, studentID))
}

func TestPaymentGetByID_ScopedByTutor(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tutorID, studentID := seedTutorStudent(t, pool)
	otherTutor, _ := seedTutorStudent(t, pool)
	courseID := addIndividualCourse(t, pool, tutorID, studentID)
	payFor(t, pool, courseID, studentID, 8)

	var paymentID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM payments WHERE course_id = $1`, courseID).Scan(&paymentID))

	repo := repository.NewPaymentRepository(pool)
	p, err := repo.GetByID(ctx, paymentID, tutorID)
	require.NoError(t, err)
	require.NotNil(t, p.StudentID)
	assert.Equal(t, studentID, *p.StudentID)

	_, err = repo.GetByID(ctx, paymentID, otherTutor)
	assert.True(t, errors.Is(err, pgx.ErrNoRows))
}

// История курса показывает адресата платежа группы, а легаси-платёж — без него.
func TestPaymentGetByCourse_NameComesFromAddressee(t *testing.T) {
	pool := testPool(t)
	tutorID, studentID := seedTutorStudent(t, pool)
	groupID := addGroupCourse(t, pool, tutorID, "Группа")
	enroll(t, pool, groupID, studentID, "NOW() - interval '1 month'")
	payFor(t, pool, groupID, studentID, 4)
	addPayment(t, pool, groupID, 5000, 4, "NOW() - interval '1 day'")

	list, total, err := repository.NewPaymentRepository(pool).
		GetByCourse(context.Background(), groupID, models.Pagination{Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, list, 2)

	require.NotNil(t, list[0].StudentName)
	assert.Equal(t, "Иван Петров", *list[0].StudentName)
	assert.Nil(t, list[1].StudentID)
	assert.Nil(t, list[1].StudentName)
}
