'use client'

import { Suspense, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'

import { studentLoginSchema, StudentLoginInput } from '@/schemas/studentAuth'
import { studentApi } from '@/lib/api/student'
import { useStudentAuthStore } from '@/stores/studentAuth'
import { ApiError } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

function LoginForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const setAuth = useStudentAuthStore((s) => s.setAuth)
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<StudentLoginInput>({ resolver: zodResolver(studentLoginSchema) })

  async function onSubmit(values: StudentLoginInput) {
    setLoading(true)
    try {
      const { access_token } = await studentApi.login(values)
      // Токен кладём до me(): request-interceptor подхватит его для запроса профиля
      localStorage.setItem('tg_student_token', access_token)
      const user = await studentApi.me()
      setAuth(access_token, user)
      const next = searchParams.get('next')
      // только внутренние пути кабинета — без open redirect
      router.replace(next && next.startsWith('/student') ? next : '/student/lessons')
    } catch (err) {
      toast.error((err as ApiError).status === 401 ? 'Неверный логин или пароль' : ((err as ApiError).message ?? 'Ошибка входа'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Вход для ученика</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Аккаунт создаётся по приглашению репетитора
      </p>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="identifier">Телефон или логин</Label>
          <Input
            id="identifier"
            placeholder="+77001234567 или логин"
            autoComplete="username"
            {...register('identifier')}
          />
          {errors.identifier && (
            <p className="text-xs text-destructive">{errors.identifier.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <Input
            id="password"
            type="password"
            placeholder="••••••"
            autoComplete="current-password"
            {...register('password')}
          />
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Вход...' : 'Войти'}
        </Button>
      </form>
    </div>
  )
}

export default function StudentLoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  )
}
