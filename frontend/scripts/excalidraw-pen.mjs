// Добавляет «ровное перо» — freedraw, толщина которого не зависит от скорости.
//
// Зачем патч, а не пропс: Excalidraw решает про давление одной строкой при
// создании элемента (`simulatePressure = event.pressure === 0.5`) и наружу это
// не отдаёт. Всё остальное уже работает само:
//   - simulatePressure — поле элемента, едет по WS → ученик видит то же перо,
//     протокол синка не трогаем;
//   - при simulatePressure:false рендерер берёт element.pressures[i], а мышь
//     всегда шлёт pressure 0.5 → массив констант → ровная линия.
// Толщину при этом задаёт appState.currentItemStrokeWidth (обычный number, не
// enum), поэтому перо тоньше «тонкого» делается без патча — см. ExcalidrawCanvas.tsx.
//
// Переключатель — globalThis.__uniformPen, его дёргает кнопка на доске.
import { readFile, writeFile } from 'node:fs/promises'

// ponytail: sed по dist вместо patch-package — в проекте уже есть postinstall,
// правящий node_modules (см. excalidraw-fonts.mjs). Станет патчей больше двух —
// заводить patch-package, он хранит diff в git и виден на ревью.
const FLAG = 'globalThis.__uniformPen'

// Bundler'ы минифицируют по-разному, поэтому цель описана дважды. dev-сборку
// берёт `next dev` (exports condition "development"), prod — сборка.
const TARGETS = [
  {
    file: 'node_modules/@excalidraw/excalidraw/dist/dev/index.js',
    from: 'const simulatePressure = event.pressure === 0.5;',
    to: `const simulatePressure = ${FLAG} ? false : event.pressure === 0.5;`,
  },
  {
    file: 'node_modules/@excalidraw/excalidraw/dist/prod/index.js',
    from: 's=t.pressure===.5',
    to: `s=${FLAG}?!1:t.pressure===.5`,
  },
]

for (const { file, from, to } of TARGETS) {
  const src = await readFile(file, 'utf8')

  if (src.includes(FLAG)) {
    console.log(`[pen] ${file} — уже пропатчен`)
    continue
  }

  // Строка захардкожена в бандле и при апгрейде пакета может уехать или
  // переминифицироваться. Без этой проверки sed молча стал бы no-op, и «ровное
  // перо» тихо превратилось бы в обычное. Падаем громко.
  const hits = src.split(from).length - 1
  if (hits !== 1) {
    console.error(`[pen] в ${file} ожидалось 1 вхождение "${from}", найдено ${hits}`)
    console.error('[pen] похоже на апгрейд @excalidraw/excalidraw — сверь строку с dist')
    process.exit(1)
  }

  await writeFile(file, src.replace(from, to))
  console.log(`[pen] ${file} ← ${FLAG}`)
}
