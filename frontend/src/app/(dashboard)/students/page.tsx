import { redirect } from 'next/navigation'

export default function StudentsPage() {
  redirect('/courses?tab=students')
}
