package db

import (
	"context"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
)

func CreatePayment(conn *pgx.Conn, req models.CreatePaymentRequest) (models.Payment, error) {
	var payment models.Payment
	err := conn.QueryRow(context.Background(),
		`INSERTO INTO payments (course_id, amount, lessons_count, paid_at)
VALUES ($1, $2, $3, $4)
RETURNING id, course_id, amount, lessons_count, paid_at`,
		req.CourseID, req.Amount, req.LessonsCount, req.PaidAt,
	).Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
	return payment, err
}

func GetPaymentsByCourse(conn *pgx.Conn, courseID string) ([]models.Payment, error) {
	rows, err := conn.Query(context.Background(),
		`SELECT id, course_id, amount, lessons_count, paid_at
		 FROM payments WHERE course_id = $1`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var payment models.Payment
		err := rows.Scan(&payment.ID, &payment.CourseID, &payment.Amount, &payment.LessonsCount, &payment.PaidAt)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, nil
}

func GetCourseBalance(conn *pgx.Conn, courseID string) (int, error) {
	var balance int
	err := conn.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(lessons_count), 0) FROM payments WHERE course_id = $1`, courseID,
	).Scan(&balance)
	return balance, err
}
