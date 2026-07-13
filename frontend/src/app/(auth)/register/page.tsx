'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import Link from 'next/link'
import { toast } from 'sonner'
import { Eye, EyeOff } from 'lucide-react'

import { registerSchema, RegisterInput } from '@/schemas/auth'
import { authApi } from '@/lib/api/auth'
import { tutorsApi } from '@/lib/api/tutors'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

// Инпут пароля с нативным toggle «показать/скрыть» — без библиотек.
function PasswordInput(props: React.ComponentProps<typeof Input>) {
  const [show, setShow] = useState(false)
  return (
    <div className="relative">
      <Input {...props} type={show ? 'text' : 'password'} className="pr-9" />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        aria-label={show ? 'Скрыть пароль' : 'Показать пароль'}
        className="absolute inset-y-0 right-0 flex items-center px-2.5 text-muted-foreground hover:text-foreground"
      >
        {show ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
      </button>
    </div>
  )
}

export default function RegisterPage() {
  const router = useRouter()
  const setAuth = useAuthStore((s) => s.setAuth)

  const [step, setStep] = useState<'form' | 'code'>('form')
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<RegisterInput>({ resolver: zodResolver(registerSchema) })

  async function loginWithToken(accessToken: string) {
    localStorage.setItem('tg_token', accessToken)
    const { id } = JSON.parse(atob(accessToken.split('.')[1]))
    const user = await tutorsApi.get(id)
    setAuth(accessToken, user)
    router.replace('/dashboard')
  }

  async function onSubmitForm(values: RegisterInput) {
    setLoading(true)
    try {
      await authApi.register({
        email: values.email,
        password: values.password,
        first_name: values.first_name,
        last_name: values.last_name,
        phone: values.phone || undefined,
      })
      setEmail(values.email)
      setStep('code')
      toast.success(`Код отправлен на ${values.email}`)
    } catch (err) {
      const e = err as ApiError
      toast.error(e.message ?? 'Ошибка регистрации')
    } finally {
      setLoading(false)
    }
  }

  if (step === 'code') {
    return <CodeStep email={email} onDone={loginWithToken} onBack={() => setStep('form')} />
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

      <form onSubmit={handleSubmit(onSubmitForm)} className="space-y-4">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="first_name">Имя</Label>
            <Input id="first_name" {...register('first_name')} />
            {errors.first_name && (
              <p className="text-xs text-destructive">{errors.first_name.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="last_name">Фамилия</Label>
            <Input id="last_name" {...register('last_name')} />
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
          {errors.email && <p className="text-xs text-destructive">{errors.email.message}</p>}
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
          {errors.phone && <p className="text-xs text-destructive">{errors.phone.message}</p>}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <PasswordInput
            id="password"
            placeholder="Минимум 6 символов"
            autoComplete="new-password"
            {...register('password')}
          />
          {errors.password && (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="confirm_password">Повторите пароль</Label>
          <PasswordInput
            id="confirm_password"
            placeholder="Ещё раз"
            autoComplete="new-password"
            {...register('confirm_password')}
          />
          {errors.confirm_password && (
            <p className="text-xs text-destructive">{errors.confirm_password.message}</p>
          )}
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Отправка...' : 'Продолжить'}
        </Button>
      </form>
    </div>
  )
}

function CodeStep({
  email,
  onDone,
  onBack,
}: {
  email: string
  onDone: (accessToken: string) => Promise<void>
  onBack: () => void
}) {
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [cooldown, setCooldown] = useState(0)

  async function verify(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      const { access_token } = await authApi.registerVerify(email, code)
      await onDone(access_token)
    } catch (err) {
      const ex = err as ApiError
      toast.error(ex.message ?? 'Неверный код')
      setLoading(false)
    }
  }

  async function resend() {
    try {
      await authApi.registerResend(email)
      toast.success('Код отправлен повторно')
      setCooldown(60)
      const timer = setInterval(() => {
        setCooldown((s) => {
          if (s <= 1) {
            clearInterval(timer)
            return 0
          }
          return s - 1
        })
      }, 1000)
    } catch (err) {
      toast.error((err as ApiError).message ?? 'Не удалось отправить код')
    }
  }

  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <h1 className="text-2xl font-semibold tracking-tight mb-1">Подтверждение</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Код отправлен на <span className="font-medium text-foreground">{email}</span>
      </p>

      <form onSubmit={verify} className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="code">Код из письма</Label>
          <Input
            id="code"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            placeholder="123456"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
            className="tracking-[0.4em] text-center text-lg"
          />
        </div>

        <Button type="submit" className="w-full" disabled={loading || code.length !== 6}>
          {loading ? 'Проверка...' : 'Подтвердить'}
        </Button>
      </form>

      <div className="mt-4 flex items-center justify-between text-sm">
        <button
          type="button"
          onClick={onBack}
          className="text-muted-foreground hover:text-foreground"
        >
          ← Назад
        </button>
        <button
          type="button"
          onClick={resend}
          disabled={cooldown > 0}
          className="text-primary hover:underline disabled:text-muted-foreground disabled:no-underline"
        >
          {cooldown > 0 ? `Отправить ещё раз (${cooldown})` : 'Отправить ещё раз'}
        </button>
      </div>
    </div>
  )
}
