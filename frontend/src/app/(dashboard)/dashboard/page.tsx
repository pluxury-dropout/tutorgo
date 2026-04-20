import { PageHeader } from '@/components/common/PageHeader'

export default function DashboardPage() {
  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Обзор на сегодня"
      />
      <p className="text-sm text-muted-foreground">— карточки со статистикой будут здесь —</p>
    </>
  )
}
