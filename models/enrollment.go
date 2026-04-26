package models

type CourseEnrollment struct {
	ID               string `json:"id"`
	CourseID         string `json:"course_id"`
	StudentID        string `json:"student_id"`
	StudentFirstName string `json:"student_first_name"`
	StudentLastName  string `json:"student_last_name"`
}

type EnrollStudentRequest struct {
	StudentID string `json:"student_id" validate:"required,uuid"`
}
