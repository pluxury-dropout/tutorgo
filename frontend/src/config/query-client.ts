import { QueryClient, QueryCache } from '@tanstack/react-query'
import { toast } from 'sonner'
import { shouldToastQueryError } from '@/lib/queryError'

// Сеть страховки под ErrorState: тот закрывает списки, а сюда попадает всё
// остальное — баланс, профиль, вложенные карточки. Без этого упавший запрос
// не оставляет вообще никакого следа в интерфейсе.
const queryCache = new QueryCache({
  onError: (error) => {
    if (!shouldToastQueryError(error)) return
    // Общий id: при веерном падении нескольких запросов покажется один тост,
    // а не стопка одинаковых.
    toast.error('Не удалось загрузить данные', { id: 'query-error' })
  },
})

export const queryClient = new QueryClient({
  queryCache,
  defaultOptions: {
    queries: {
      staleTime: 2 * 60_000,
      gcTime: 10 * 60_000,
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})
