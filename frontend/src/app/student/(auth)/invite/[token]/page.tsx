'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { acceptInviteSchema, AcceptInviteInput } from '@/schemas/studentAuth'
import { studentApi } from '@/lib/api/student'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { ApiError } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function AcceptInvitePage() {
  const { token } = useParams<{ token: string }>()
  const router = useRouter()
  const setAuth = useStudentAuthStore((s) => s.setAuth)
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<AcceptInviteInput>({ resolver: zodResolver(acceptInviteSchema) })

  async function onSubmit(values: AcceptInviteInput) {
    setLoading(true)
    try {
      const { access_token } = await studentApi.acceptInvite({
        token,
        username: values.username,
        password: values.password,
      })
      localStorage.setItem('tg_student_token', access_token)
      const user = await studentApi.me()
      setAuth(access_token, user)
      router.replace('/student/lessons')
    } catch (err) {
      const e = err as ApiError
      if (e.status === 409) {
        setError('username', { message: 'Такой логин или телефон уже занят' })
      } else if (e.status === 404 || e.status === 400) {
        toast.error('Ссылка недействительна или истекла. Попросите репетитора прислать новую')
      } else {
        toast.error(e.message ?? 'Не удалось создать аккаунт')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Создание аккаунта</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Придумайте логин и пароль для входа в кабинет ученика
      </p>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="username">Логин</Label>
          <Input id="username" placeholder="латинские буквы и цифры" autoComplete="username" {...register('username')} />
          {errors.username && (
            <p className="text-xs text-destructive">{errors.username.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <Input id="password" type="password" placeholder="••••••" autoComplete="new-password" {...register('password')} />
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="confirm">Повторите пароль</Label>
          <Input id="confirm" type="password" placeholder="••••••" autoComplete="new-password" {...register('confirm')} />
          {errors.confirm && (
            <p className="text-xs text-destructive">{errors.confirm.message}</p>
          )}
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Создание...' : 'Создать аккаунт'}
        </Button>
      </form>
    </div>
  )
}
