import { StudentGate } from '@/components/student/StudentGate'

export default function StudentRoomLayout({ children }: { children: React.ReactNode }) {
  return <StudentGate>{children}</StudentGate>
}
