'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'

import { useAuthStore } from '@/stores/auth'
import { useTutor, useUpdateTutor, useEnsureIcsLink, useRevokeIcsLink } from '@/lib/hooks/useTutor'
import { tutorProfileSchema, TutorProfileValues, changePasswordSchema, ChangePasswordValues } from '@/schemas/tutor'
import { tutorsApi, icsFeedUrl } from '@/lib/api/tutors'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

function initials(firstName?: string, lastName?: string) {
  return `${(firstName?.[0] ?? '').toUpperCase()}${(lastName?.[0] ?? '').toUpperCase()}`
}

export default function ProfilePage() {
  const { user, token, setAuth } = useAuthStore()
  const { data: tutor }          = useTutor(user?.id ?? '')
  const { mutate: updateProfile, isPending: savingProfile } = useUpdateTutor()
  const { mutate: changePassword, isPending: savingPassword } = useMutation({
    mutationFn: ({ current_password, new_password }: ChangePasswordValues) =>
      tutorsApi.changePassword(user!.id, { current_password, new_password }),
  })

  // Токен живёт только в state страницы: GET-ручки «а есть ли ссылка» нет
  // (сознательно, см. handlers/ics.go), а дёргать EnsureLink молча при заходе
  // на страницу значило бы выпускать подписку всем, кто просто открыл профиль.
  const [icsToken, setIcsToken] = useState<string | null>(null)
  const ensureIcsLink = useEnsureIcsLink()
  const revokeIcsLink = useRevokeIcsLink()

  function handleCreateIcsLink() {
    ensureIcsLink.mutate(undefined, {
      onSuccess: setIcsToken,
      onError: () => toast.error('Не удалось создать ссылку'),
    })
  }

  function handleCopyIcsLink() {
    if (!icsToken) return
    navigator.clipboard.writeText(icsFeedUrl(icsToken))
    toast.success('Ссылка скопирована')
  }

  function handleRevokeIcsLink() {
    if (!confirm('Отозвать ссылку? Все, кто на неё подписан в Google Календаре или на телефоне, перестанут получать обновления расписания.')) return
    revokeIcsLink.mutate(undefined, {
      onSuccess: () => {
        setIcsToken(null)
        toast.success('Ссылка отозвана')
      },
      onError: () => toast.error('Не удалось отозвать ссылку'),
    })
  }

  const profileForm = useForm<TutorProfileValues>({
    resolver: zodResolver(tutorProfileSchema),
  })

  const passwordForm = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
  })

  useEffect(() => {
    if (tutor) profileForm.reset({
      first_name: tutor.first_name,
      last_name:  tutor.last_name,
      email:      tutor.email,
      phone:      tutor.phone ?? '',
    })
  }, [tutor, profileForm.reset])

  function onProfileSubmit(values: TutorProfileValues) {
    if (!user) return
    updateProfile(
      { id: user.id, data: values },
      {
        onSuccess: (updated) => {
          setAuth(token!, updated)
          toast.success('Профиль обновлён')
        },
        onError: () => toast.error('Не удалось сохранить'),
      },
    )
  }

  function onPasswordSubmit(values: ChangePasswordValues) {
    changePassword(values, {
      onSuccess: () => {
        passwordForm.reset()
        toast.success('Пароль изменён')
      },
      onError: (err: any) => {
        const msg = err?.response?.data?.error ?? 'Не удалось изменить пароль'
        toast.error(msg)
      },
    })
  }

  return (
    <>
      <PageHeader title="Профиль" />

      <div className="mt-6 max-w-lg space-y-6">
        {/* Avatar */}
        <div className="flex items-center gap-4">
          <div className="h-16 w-16 rounded-full bg-primary flex items-center justify-center text-primary-foreground text-xl font-semibold shrink-0">
            {initials(tutor?.first_name, tutor?.last_name)}
          </div>
          <div>
            <p className="font-semibold text-base">
              {tutor ? `${tutor.first_name} ${tutor.last_name}` : '—'}
            </p>
            <p className="text-sm text-muted-foreground">{tutor?.email}</p>
          </div>
        </div>

        {/* Profile form */}
        <form onSubmit={profileForm.handleSubmit(onProfileSubmit)} className="border rounded-xl bg-card p-5 space-y-4">
          <h2 className="text-sm font-semibold">Личные данные</h2>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label>Имя</Label>
              <Input {...profileForm.register('first_name')} />
              {profileForm.formState.errors.first_name && (
                <p className="text-xs text-destructive">{profileForm.formState.errors.first_name.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label>Фамилия</Label>
              <Input {...profileForm.register('last_name')} />
              {profileForm.formState.errors.last_name && (
                <p className="text-xs text-destructive">{profileForm.formState.errors.last_name.message}</p>
              )}
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Email</Label>
            <Input type="email" {...profileForm.register('email')} />
            {profileForm.formState.errors.email && (
              <p className="text-xs text-destructive">{profileForm.formState.errors.email.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label>Телефон</Label>
            <Input type="tel" {...profileForm.register('phone')} />
            {profileForm.formState.errors.phone && (
              <p className="text-xs text-destructive">{profileForm.formState.errors.phone.message}</p>
            )}
          </div>

          <Button type="submit" disabled={savingProfile}>
            {savingProfile ? 'Сохраняю...' : 'Сохранить'}
          </Button>
        </form>

        {/* Password form */}
        <form onSubmit={passwordForm.handleSubmit(onPasswordSubmit)} className="border rounded-xl bg-card p-5 space-y-4">
          <h2 className="text-sm font-semibold">Смена пароля</h2>

          <div className="space-y-1.5">
            <Label>Текущий пароль</Label>
            <Input type="password" {...passwordForm.register('current_password')} />
            {passwordForm.formState.errors.current_password && (
              <p className="text-xs text-destructive">{passwordForm.formState.errors.current_password.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label>Новый пароль</Label>
            <Input type="password" {...passwordForm.register('new_password')} />
            {passwordForm.formState.errors.new_password && (
              <p className="text-xs text-destructive">{passwordForm.formState.errors.new_password.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label>Повторите новый пароль</Label>
            <Input type="password" {...passwordForm.register('confirm_password')} />
            {passwordForm.formState.errors.confirm_password && (
              <p className="text-xs text-destructive">{passwordForm.formState.errors.confirm_password.message}</p>
            )}
          </div>

          <Button type="submit" disabled={savingPassword}>
            {savingPassword ? 'Сохраняю...' : 'Изменить пароль'}
          </Button>
        </form>

        {/* Подписка */}
        <div className="border rounded-xl bg-card p-5 flex items-center justify-between">
          <div>
            <h2 className="text-sm font-semibold">Подписка</h2>
            <p className="text-xs text-muted-foreground mt-0.5">Тариф и оплата доступа</p>
          </div>
          <Link href="/subscription" className="text-sm text-primary underline underline-offset-2">
            Управлять
          </Link>
        </div>

        {/* Подписка на календарь (ICS-фид) */}
        <div className="border rounded-xl bg-card p-5 space-y-3">
          <div>
            <h2 className="text-sm font-semibold">Подписка на календарь</h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              Расписание можно открыть в Google Календаре или на телефоне. Синхронизация
              односторонняя — только чтение, править занятия по-прежнему нужно здесь.
            </p>
          </div>

          {icsToken ? (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Input readOnly value={icsFeedUrl(icsToken)} className="font-mono text-xs truncate" />
                <Button type="button" variant="outline" size="sm" onClick={handleCopyIcsLink}>
                  Копировать
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Google Календарь: «Другие календари» → «Подписаться по URL» → вставьте ссылку.
              </p>
              <Button
                type="button"
                variant="destructive"
                size="sm"
                onClick={handleRevokeIcsLink}
                disabled={revokeIcsLink.isPending}
              >
                Отозвать
              </Button>
            </div>
          ) : (
            <Button type="button" variant="outline" size="sm" onClick={handleCreateIcsLink} disabled={ensureIcsLink.isPending}>
              {ensureIcsLink.isPending ? 'Создаю...' : 'Создать ссылку'}
            </Button>
          )}
        </div>
      </div>
    </>
  )
}
