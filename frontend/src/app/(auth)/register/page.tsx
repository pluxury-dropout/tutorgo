'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import Link from 'next/link'
import { toast } from 'sonner'

import { registerSchema, RegisterInput } from '@/schemas/auth'
import { authApi } from '@/lib/api/auth'
import { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function RegisterPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<RegisterInput>({ resolver: zodResolver(registerSchema) })

  async function onSubmit(values: RegisterInput) {
    setLoading(true)
    try {
      await authApi.register({
        ...values,
        phone: values.phone || undefined,
      })
      toast.success('Аккаунт создан — войдите')
      router.push('/login')
    } catch (err) {
      const e = err as ApiError
      if (e.fieldErrors) {
        toast.error(Object.values(e.fieldErrors).join(', '))
      } else {
        toast.error(e.message ?? 'Ошибка регистрации')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Регистрация</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Уже есть аккаунт?{' '}
        <Link href="/login" className="text-primary hover:underline">
          Войти
        </Link>
      </p>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="first_name">Имя</Label>
            <Input id="first_name" placeholder="Айгерим" {...register('first_name')} />
            {errors.first_name && (
              <p className="text-xs text-destructive">{errors.first_name.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="last_name">Фамилия</Label>
            <Input id="last_name" placeholder="Бекова" {...register('last_name')} />
            {errors.last_name && (
              <p className="text-xs text-destructive">{errors.last_name.message}</p>
            )}
          </div>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            placeholder="tutor@example.com"
            autoComplete="email"
            {...register('email')}
          />
          {errors.email && (
            <p className="text-xs text-destructive">{errors.email.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="phone">
            Телефон{' '}
            <span className="text-muted-foreground font-normal">(необязательно)</span>
          </Label>
          <Input
            id="phone"
            type="tel"
            placeholder="+77001234567"
            autoComplete="tel"
            {...register('phone')}
          />
          {errors.phone && (
            <p className="text-xs text-destructive">{errors.phone.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <Input
            id="password"
            type="password"
            placeholder="Минимум 6 символов"
            autoComplete="new-password"
            {...register('password')}
          />
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Создание...' : 'Создать аккаунт'}
        </Button>
      </form>
    </div>
  )
}
