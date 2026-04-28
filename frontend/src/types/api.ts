export interface Tutor {
  id: string
  email: string
  first_name: string
  last_name: string
  phone: string
}

export interface Student {
  id: string
  first_name: string
  last_name: string | null
  email: string
  phone: string
  tutor_id: string
}

export interface Course {
  id: string
  student_id: string | null
  tutor_id: string
  subject: string
  price_per_lesson: number
  started_at: string
  ended_at: string | null
}

export interface CourseBalance {
  lessons_paid: number
  lessons_completed: number
  lessons_remaining: number
}

export type LessonStatus = 'scheduled' | 'completed' | 'cancelled' | 'missed'

export interface Lesson {
  id: string
  course_id: string
  scheduled_at: string
  duration_minutes: number
  status: LessonStatus
  notes: string
  series_id?: string
}

export interface CalendarLesson {
  id: string
  course_id: string
  scheduled_at: string
  duration_minutes: number
  status: LessonStatus
  notes: string
  subject: string
  student_name: string | null
  is_group: boolean
}

export interface Payment {
  id: string
  course_id: string
  amount: number
  lessons_count: number
  paid_at: string
}

export interface PaymentBalance {
  total: number
}

export interface Enrollment {
  course_id: string
  student_id: string
  student_first_name: string
  student_last_name: string | null
}

export type AttendanceStatus = 'present' | 'absent'

export interface AttendanceRecord {
  lesson_id: string
  student_id: string
  status: AttendanceStatus
}

export type ApiValidationError = Record<string, string>
export interface ApiError {
  message: string
  fieldErrors?: ApiValidationError
  status: number
}
