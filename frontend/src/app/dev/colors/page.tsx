"use client"

import { useEffect, useState } from "react"

const SECTIONS = [
  {
    title: "Base",
    tokens: [
      "background", "foreground",
      "card", "card-foreground",
      "popover", "popover-foreground",
      "primary", "primary-light", "primary-foreground",
      "secondary", "secondary-foreground",
      "muted", "muted-foreground",
      "accent", "accent-foreground", "accent-light",
      "destructive",
      "border", "input", "ring",
    ],
  },
  {
    title: "Semantic",
    tokens: ["success", "warning", "purple", "danger"],
  },
  {
    title: "Sidebar",
    tokens: [
      "sidebar", "sidebar-foreground",
      "sidebar-primary", "sidebar-primary-foreground",
      "sidebar-accent", "sidebar-accent-foreground",
      "sidebar-border", "sidebar-ring",
      "sidebar-active-bg", "sidebar-active-text",
      "sidebar-hover-bg", "sidebar-text",
    ],
  },
  {
    title: "Status chips",
    tokens: [
      "status-scheduled-bg", "status-scheduled-text",
      "status-completed-bg", "status-completed-text",
      "status-cancelled-bg", "status-cancelled-text",
      "status-missed-bg", "status-missed-text",
    ],
  },
  {
    title: "Calendar chips",
    tokens: [
      "cal-scheduled-bg", "cal-scheduled-border", "cal-scheduled-text",
      "cal-completed-bg", "cal-completed-border", "cal-completed-text",
      "cal-cancelled-bg", "cal-cancelled-border", "cal-cancelled-text",
      "cal-missed-bg", "cal-missed-border", "cal-missed-text",
    ],
  },
  {
    title: "Charts",
    tokens: ["chart-1", "chart-2", "chart-3", "chart-4", "chart-5"],
  },
]

function resolveTokens(tokens: string[]): Record<string, string> {
  const style = getComputedStyle(document.documentElement)
  return Object.fromEntries(
    tokens.map((t) => [t, style.getPropertyValue(`--${t}`).trim()])
  )
}

function ColorSwatch({ token, value }: { token: string; value: string }) {
  return (
    <div className="flex items-center gap-3 py-2">
      <div
        className="h-10 w-10 rounded-md border border-black/10 shrink-0"
        style={{ background: `var(--${token})` }}
      />
      <div className="min-w-0">
        <p className="font-mono text-xs text-foreground">--{token}</p>
        <p className="font-mono text-[11px] text-muted-foreground">{value || "—"}</p>
      </div>
    </div>
  )
}

export default function ColorsPage() {
  const [dark, setDark] = useState(false)
  const [resolved, setResolved] = useState<Record<string, string>>({})

  useEffect(() => {
    const saved = localStorage.getItem("theme")
    if (saved === "dark") {
      document.documentElement.classList.add("dark")
      setDark(true)
    }
  }, [])

  useEffect(() => {
    const allTokens = SECTIONS.flatMap((s) => s.tokens)
    setResolved(resolveTokens(allTokens))
  }, [dark])

  function toggleTheme() {
    const next = !dark
    setDark(next)
    if (next) {
      document.documentElement.classList.add("dark")
      localStorage.setItem("theme", "dark")
    } else {
      document.documentElement.classList.remove("dark")
      localStorage.setItem("theme", "light")
    }
  }

  return (
    <div className="min-h-screen bg-background px-8 py-10">
      <div className="max-w-4xl mx-auto">
        <div className="flex items-center justify-between mb-10">
          <h1 className="text-2xl font-bold text-foreground">Color Tokens</h1>
          <button
            onClick={toggleTheme}
            className="px-3 py-1.5 rounded-md border border-border text-sm text-foreground hover:bg-muted transition-colors"
            aria-label="Toggle dark theme"
          >
            {dark ? "☀ Light" : "🌙 Dark"}
          </button>
        </div>

        <div className="space-y-12">
          {SECTIONS.map((section) => (
            <section key={section.title}>
              <h2 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-4 border-b border-border pb-2">
                {section.title}
              </h2>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-x-6">
                {section.tokens.map((token) => (
                  <ColorSwatch
                    key={token}
                    token={token}
                    value={resolved[token] ?? ""}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  )
}
