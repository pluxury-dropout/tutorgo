import { create } from 'zustand'

type CalendarView = 'timeGridWeek' | 'dayGridMonth'

interface UIState {
  sidebarCollapsed: boolean
  calendarView: CalendarView
  calendarDate: string
  toggleSidebar: () => void
  setCalendarView: (view: CalendarView) => void
  setCalendarDate: (date: string) => void
}

export const useUIStore = create<UIState>((set) => ({
  sidebarCollapsed: false,
  calendarView: 'timeGridWeek',
  calendarDate: new Date().toISOString().split('T')[0],

  toggleSidebar: () =>
    set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
  setCalendarView: (view) => set({ calendarView: view }),
  setCalendarDate: (date) => set({ calendarDate: date }),
}))
