package db

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

func CreateCourse(conn *pgx.Conn, req models.CreateCourseRequest, tutorID string) (models.Course, error) {
	var course models.Course
	err := conn.QueryRow(context.Background(),
		`INSERT INTO courses (student_id, tutor_id, subject, price_per_lesson, started_at, ended_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at`,
		req.StudentID, tutorID, req.Subject, req.PricePerLesson, req.StartedAt, req.EndedAt,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func GetCourses(conn *pgx.Conn, tutorID string) ([]models.Course, error) {
	rows, err := conn.Query(context.Background(),
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
	FROM courses WHERE tutor_id=$1`, tutorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []models.Course
	for rows.Next() {
		var course models.Course
		err := rows.Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, nil
}

func GetCourseByID(conn *pgx.Conn, id string, tutorID string) (models.Course, error) {
	var course models.Course
	err := conn.QueryRow(context.Background(),
		`SELECT id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at
		 FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func UpdateCourse(conn *pgx.Conn, id string, tutorID string, req models.UpdateCourseRequest) (models.Course, error) {
	var course models.Course
	err := conn.QueryRow(context.Background(),
		`UPDATE courses SET subject=$1, price_per_lesson=$2, started_at=$3, ended_at=$4
		 WHERE id=$5 AND tutor_id=$6
		 RETURNING id, student_id, tutor_id, subject, price_per_lesson, started_at, ended_at`,
		req.Subject, req.PricePerLesson, req.StartedAt, req.EndedAt, id, tutorID,
	).Scan(&course.ID, &course.StudentID, &course.TutorID, &course.Subject, &course.PricePerLesson, &course.StartedAt, &course.EndedAt)
	return course, err
}

func DeleteCourse(conn *pgx.Conn, id string, tutorID string) error {
	_, err := conn.Exec(context.Background(),
		`DELETE FROM courses WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}
