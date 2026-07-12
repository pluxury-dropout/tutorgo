'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'

import { studentApi } from '@/lib/api/student'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { ApiError } from '@/types/api'
import { SectionCard, SectionRow } from '@/components/common/SectionCard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const changePasswordSchema = z
  .object({
    old_password: z.string().min(6, 'Минимум 6 символов'),
    new_password: z.string().min(6, 'Минимум 6 символов'),
    confirm: z.string(),
  })
  .refine((d) => d.new_password === d.confirm, {
    path: ['confirm'],
    message: 'Пароли не совпадают',
  })

type ChangePasswordInput = z.infer<typeof changePasswordSchema>

export default function StudentProfilePage() {
  const setAuth = useStudentAuthStore((s) => s.setAuth)

  const { data: profile, isLoading } = useQuery({
    queryKey: ['student-me'],
    queryFn: studentApi.me,
  })

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ChangePasswordInput>({ resolver: zodResolver(changePasswordSchema) })

  async function onSubmit(values: ChangePasswordInput) {
    try {
      const { access_token } = await studentApi.changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      })
      // Backend отозвал ВСЕ refresh-сессии и выдал новую текущему устройству.
      // Без сохранения свежего токена следующий запрос уйдёт с мёртвой сессией.
      localStorage.setItem('tg_student_token', access_token)
      if (profile) setAuth(access_token, profile)
      toast.success('Пароль изменён. Другие устройства разлогинены')
      reset()
    } catch (err) {
      const e = err as ApiError
      if (e.status === 401) {
        setError('old_password', { message: 'Неверный текущий пароль' })
      } else {
        toast.error(e.message ?? 'Не удалось сменить пароль')
      }
    }
  }

  return (
    <div className="space-y-4">
      <SectionCard title="Профиль">
        {isLoading && (
          <SectionRow isFirst>
            <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>Загрузка...</span>
          </SectionRow>
        )}
        {profile && (
          <>
            <SectionRow isFirst>
              <ProfileField label="Имя" value={`${profile.first_name} ${profile.last_name}`.trim()} />
            </SectionRow>
            <SectionRow>
              <ProfileField label="Телефон" value={profile.phone || '—'} />
            </SectionRow>
            <SectionRow>
              <ProfileField label="Логин" value={profile.username || '—'} />
            </SectionRow>
          </>
        )}
      </SectionCard>

      <SectionCard title="Смена пароля" bodyPadding={18}>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 max-w-sm">
          <div className="space-y-1.5">
            <Label htmlFor="old_password">Текущий пароль</Label>
            <Input id="old_password" type="password" autoComplete="current-password" {...register('old_password')} />
            {errors.old_password && (
              <p className="text-xs text-destructive">{errors.old_password.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="new_password">Новый пароль</Label>
            <Input id="new_password" type="password" autoComplete="new-password" {...register('new_password')} />
            {errors.new_password && (
              <p className="text-xs text-destructive">{errors.new_password.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="confirm">Повторите новый пароль</Label>
            <Input id="confirm" type="password" autoComplete="new-password" {...register('confirm')} />
            {errors.confirm && (
              <p className="text-xs text-destructive">{errors.confirm.message}</p>
            )}
          </div>

          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Сохранение...' : 'Сменить пароль'}
          </Button>
        </form>
      </SectionCard>
    </div>
  )
}

function ProfileField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span style={{ fontSize: 13, color: 'var(--muted-foreground)' }}>{label}</span>
      <span style={{ fontSize: 13.5, fontWeight: 500 }}>{value}</span>
    </div>
  )
}
