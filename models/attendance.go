package models

type LessonAttendance struct {
	ID        string `json:"id"`
	LessonID  string `json:"lesson_id"`
	StudentID string `json:"student_id"`
	Status    string `json:"status"`
}

type AttendanceEntry struct {
	StudentID string `json:"student_id" validate:"required,uuid"`
	Status    string `json:"status"     validate:"required,oneof=present absent"`
}

type UpdateAttendanceRequest struct {
	Attendances []AttendanceEntry `json:"attendances" validate:"required,dive"`
}
