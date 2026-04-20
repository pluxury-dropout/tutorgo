'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import Link from 'next/link'
import { toast } from 'sonner'

import { loginSchema, LoginInput } from '@/schemas/auth'
import { authApi } from '@/lib/api/auth'
import { tutorsApi } from '@/lib/api/tutors'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function LoginPage() {
  const router = useRouter()
  const setAuth = useAuthStore((s) => s.setAuth)
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginInput>({ resolver: zodResolver(loginSchema) })

  async function onSubmit(values: LoginInput) {
    setLoading(true)
    try {
      const isEmail = values.credential.includes('@')
      const payload = isEmail
        ? { email: values.credential, password: values.password }
        : { phone: values.credential, password: values.password }

      const { token } = await authApi.login(payload)

      // Store token first so axios interceptor can use it for the profile request
      localStorage.setItem('tg_token', token)

      const payloadPart = token.split('.')[1]
      const { id } = JSON.parse(atob(payloadPart))
      const user = await tutorsApi.get(id)

      setAuth(token, user)
      router.replace('/dashboard')
    } catch (err) {
      const e = err as ApiError
      toast.error(e.message ?? 'Ошибка входа')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Войти</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Нет аккаунта?{' '}
        <Link href="/register" className="text-primary hover:underline">
          Зарегистрироваться
        </Link>
      </p>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="credential">Email или телефон</Label>
          <Input
            id="credential"
            placeholder="tutor@example.com или +77001234567"
            autoComplete="email"
            {...register('credential')}
          />
          {errors.credential && (
            <p className="text-xs text-destructive">{errors.credential.message}</p>
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
