package models

type CourseEnrollment struct {
	ID        string `json:"id"`
	CourseID  string `json:"course_id"`
	StudentID string `json:"student_id"`
}

type EnrollStudentRequest struct {
	StudentID string `json:"student_id" validate:"required,uuid"`
}
