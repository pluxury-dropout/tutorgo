import { CourseBalance } from '@/types/api'

/** Баланс курса — три числа (спека 2026-09-06, п. 7.2): общий и для страницы
 *  курса, и для вкладки «Обзор» карточки ученика — расхождения тут недопустимы,
 *  бейдж цикла и эта строка обязаны читать одно и то же. */
export function CourseBalanceStat({ balance }: { balance: CourseBalance }) {
  return (
    <div className="grid grid-cols-3 gap-3 text-center">
      <div>
        <p className="text-2xl font-bold">{balance.lessons_paid}</p>
        <p className="text-xs text-muted-foreground mt-1">Оплачено</p>
      </div>
      <div>
        <p className="text-2xl font-bold">{balance.lessons_completed}</p>
        <p className="text-xs text-muted-foreground mt-1">Проведено</p>
      </div>
      <div>
        <p className="text-2xl font-bold text-primary">{balance.lessons_remaining}</p>
        <p className="text-xs text-muted-foreground mt-1">Осталось</p>
      </div>
    </div>
  )
}
