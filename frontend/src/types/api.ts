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
  active: boolean
}

export interface Course {
  id: string
  student_id: string | null
  tutor_id: string
  subject: string
  price_per_cycle: number
  lessons_per_cycle: number
  started_at: string
  ended_at: string | null
  is_active: boolean
}

/** Расписание для быстрого онбординга — дни недели и стенное время из формы;
 *  правило повторения из них собирает сервис. */
export interface OnboardingSchedule {
  byweekday?:        number[] // ISO: 1=Пн … 7=Вс
  time_local:        string   // «17:00»
  tz:                string   // IANA, берём из браузера
  duration_minutes:  number
  starts_on:         string   // RFC3339
  ends_on?:          string | null
}

/** Тело POST /onboarding/student: ученик, курс и серия одним сабмитом.
 *  Обязательно только first_name — без subject уйдёт только ученик, без
 *  schedule — ученик и курс без уроков. */
export interface OnboardingStudentInput {
  first_name:          string
  phone?:              string
  subject?:            string
  price_per_cycle?:    number
  lessons_per_cycle?:  number
  schedule?:           OnboardingSchedule
}

export interface OnboardingResult {
  student:         Student
  course:          Course | null
  lessons_created: number
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
  cycle_position?: number
  cycle_size?: number
  /** Заполнен только у вхождения серии — правка тогда спрашивает область. */
  rule_id?: string
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
  cycle_position?: number
  cycle_size?: number
  paid?: boolean
  /** Заполнен только у вхождения серии — правка тогда спрашивает область. */
  rule_id?: string
}

export interface StudentHomework {
  course_id: string
  subject: string
  homework: string
}

export interface StudentCourse {
  id: string
  subject: string
}

export interface Payment {
  id: string
  course_id: string
  amount: number
  lessons_count: number
  paid_at: string
  /** Только в списках по репетитору (/payments, /payments/recent). */
  subject?: string
  /** Там же; отсутствует у групповых курсов. */
  student_name?: string | null
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

export interface Task {
  id: string
  tutor_id: string
  title: string
  scheduled_at: string | null
  duration_minutes: number | null
  status: 'not_urgent' | 'urgent' | 'very_urgent' | 'done'
  created_at: string
}

export type EventKind = 'personal' | 'work' | 'trial'

export interface Event {
  id: string
  tutor_id: string
  title: string
  kind: EventKind
  starts_at: string
  duration_minutes: number
  color: string
  location: string
  notes: string
  /** Заполнен только у вхождения серии — правка тогда спрашивает область. */
  rule_id?: string
}

/** Область правки вхождения серии: только это, это и следующие, всё правило. */
export type RecurrenceScope = 'one' | 'following' | 'all'

/** Правило повторения для урока или события. Время и длительность сервер берёт
 *  из первого вхождения, поэтому здесь их нет. */
export interface RecurrenceInput {
  freq:       'daily' | 'weekly' | 'monthly'
  interval_n?: number
  byweekday?: number[]   // ISO: 1=Пн … 7=Вс
  tz:         string     // IANA, берём из браузера
  ends_on?:   string
  max_count?: number
}

/** Строка единой ленты календаря: общие поля наверху, специфика — по типу. */
export type CalendarItem = {
  id: string
  title: string
  starts_at: string
  duration_minutes: number
} & (
  | { type: 'lesson'; lesson: CalendarLesson; event?: never; task?: never }
  | { type: 'event';  event: Event;           lesson?: never; task?: never }
  | { type: 'task';   task: Task;             lesson?: never; event?: never }
)

export type ApiValidationError = Record<string, string>
export interface ApiError {
  message: string
  fieldErrors?: ApiValidationError
  status: number
}

export interface PagedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export interface CurrentCycleInfo {
  course_id: string
  subject: string
  student_name: string | null
  progress: number
  cycle_size: number
  last_at: string
}

export interface Board {
  id: string
  course_id: string
  tutor_id: string
  created_at: string
}

export interface BoardPage {
  id: string
  board_id: string
  title: string
  position: number
  created_at: string
  updated_at: string
}

export interface BoardInvite {
  id: string
  board_id: string
  created_at: string
}

export interface BoardAsset {
  id: string
  board_id: string
  file_path: string
  mime_type: string
  size_bytes: number
  created_at: string
}

export interface BoardAssetResponse {
  id: string
  url: string
}

export interface BoardWithPages extends Board {
  pages: BoardPage[]
}

export interface PdfPreflightResponse {
  import_id: string
  num_pages: number
  page_sizes: { w: number; h: number }[] // пункты PDF
}

export interface PdfImportPageOut {
  file_id: string
  url: string // относительный, префиксуем BASE_URL
  w: number
  h: number
}

export interface PdfStartResponse {
  pages: PdfImportPageOut[]
}

export type MaterialKind = 'folder' | 'file'

/** Элемент библиотеки препода: папка (kind: 'folder') или файл. У папок
 *  mime_type пустой, size_bytes = 0. Путь в S3 наружу не отдаётся. */
export interface Material {
  id: string
  kind: MaterialKind
  name: string
  mime_type: string
  size_bytes: number
  created_at: string
}
