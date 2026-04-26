'use client'

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'

import { useAuthStore } from '@/stores/auth'
import { useTutor, useUpdateTutor } from '@/lib/hooks/useTutor'
import { tutorProfileSchema, TutorProfileValues, changePasswordSchema, ChangePasswordValues } from '@/schemas/tutor'
import { tutorsApi } from '@/lib/api/tutors'
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
        <form onSubmit={profileForm.handleSubmit(onProfileSubmit)} className="border rounded-lg p-5 space-y-4">
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
        <form onSubmit={passwordForm.handleSubmit(onPasswordSubmit)} className="border rounded-lg p-5 space-y-4">
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
      </div>
    </>
  )
}
