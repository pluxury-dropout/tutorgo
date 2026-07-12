import { redirect } from 'next/navigation'

// Анонимный гостевой вход в запланированные уроки удалён (уроки приватные,
// backend-эндпоинт guest-token больше не существует). Старые ссылки ведём на
// логин кабинета ученика.
export default function JoinLessonRedirect() {
  redirect('/student/login?next=/student/lessons')
}
