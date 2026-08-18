// LaTeX → самодостаточный SVG (глифы путями, ноль внешних ссылок).
//
// liteAdaptor не трогает DOM, поэтому один и тот же модуль работает в браузере
// и под `node --test`. Импортировать этот файл дорого (~490 КБ gzip), поэтому
// вызывающий код грузит его через `await import()`.
import { mathjax } from '@mathjax/src/js/mathjax.js'
import { TeX } from '@mathjax/src/js/input/tex.js'
import { SVG } from '@mathjax/src/js/output/svg.js'
import { liteAdaptor } from '@mathjax/src/js/adaptors/liteAdaptor.js'
import { RegisterHTMLHandler } from '@mathjax/src/js/handlers/html.js'
import type { LiteElement } from '@mathjax/src/js/adaptors/lite/Element.js'

import '@mathjax/src/js/input/tex/base/BaseConfiguration.js'
import '@mathjax/src/js/input/tex/ams/AmsConfiguration.js'
import '@mathjax/src/js/input/tex/newcommand/NewcommandConfiguration.js'
import '@mathjax/src/js/input/tex/noundefined/NoUndefinedConfiguration.js'
import '@mathjax/src/js/input/tex/color/ColorConfiguration.js'
import '@mathjax/src/js/input/tex/mathtools/MathtoolsConfiguration.js'
import '@mathjax/src/js/input/tex/physics/PhysicsConfiguration.js'
import '@mathjax/src/js/input/tex/cancel/CancelConfiguration.js'
import '@mathjax/src/js/input/tex/boldsymbol/BoldsymbolConfiguration.js'
import '@mathjax/src/js/input/tex/braket/BraketConfiguration.js'
import '@mathjax/src/js/input/tex/cases/CasesConfiguration.js'
import '@mathjax/src/js/input/tex/enclose/EncloseConfiguration.js'
import '@mathjax/src/js/input/tex/textmacros/TextMacrosConfiguration.js'
import '@mathjax/src/js/input/tex/upgreek/UpgreekConfiguration.js'
import '@mathjax/src/js/input/tex/unicode/UnicodeConfiguration.js'
import '@mathjax/src/js/input/tex/configmacros/ConfigMacrosConfiguration.js'
import '@mathjax/src/js/input/tex/mhchem/MhchemConfiguration.js'

// Урезать набор бессмысленно: 73% веса — глифы шрифта, все пакеты против
// одного base стоят +8.5% gzip. Поэтому берём функционально полный набор.
const PACKAGES = [
  'base', 'ams', 'newcommand', 'noundefined', 'color', 'mathtools', 'physics',
  'cancel', 'boldsymbol', 'braket', 'cases', 'enclose', 'textmacros',
  'upgreek', 'unicode', 'configmacros', 'mhchem',
]

const adaptor = liteAdaptor()
RegisterHTMLHandler(adaptor)

// Глифы за пределами базовой латиницы (кириллица, фрактура, скрипт) лежат в
// динамических чанках шрифта. MathJax просит их по имени модуля и до загрузки
// БРОСАЕТ retry-исключение — поэтому весь рендер идёт через handleRetriesFor.
// Шаблонная строка (а не конкатенация произвольного имени) нужна бандлеру:
// так Turbopack видит папку и нарезает чанки статически.
mathjax.asyncLoad = (name: string) => {
  const chunk = /dynamic\/([\w-]+)\.js$/.exec(name)?.[1]
  if (!chunk) return Promise.reject(new Error(`mathjax: неизвестный чанк ${name}`))
  return import(`@mathjax/mathjax-newcm-font/js/svg/dynamic/${chunk}.js`)
}

const doc = mathjax.document('', {
  InputJax: new TeX({
    packages: PACKAGES,
    // convertAsciiMathToLatex (MathLive) сама вставляет \placeholder{} для
    // недостающих аргументов (sqrt → \sqrt{\placeholder{}}). MathJax не
    // считает это ошибкой макроса, а рисует буквально «\placeholder»
    // красным текстом — макрос делает его прозрачным: голая подстановка
    // аргумента, ничего не выводящая при пустом содержимом.
    macros: { placeholder: ['{#1}', 1, ''] },
  }),
  // local — глифы уезжают в <defs> ЭТОГО же SVG. global вынес бы их в общий
  // documentwide defs, и картинка перестала бы быть самодостаточной.
  OutputJax: new SVG({ fontCache: 'local' }),
})

export type LatexSvg = {
  svg: string
  /** px при переданном кегле */
  width: number
  height: number
  /** текст ошибки MathJax; при наличии SVG вставлять на доску НЕЛЬЗЯ */
  error?: string
}

export async function latexToSvg(
  latex: string,
  { fontSize = 20, color = '#1e1e1e' }: { fontSize?: number; color?: string } = {}
): Promise<LatexSvg> {
  // doc.convert() типизирован как MmlNode | N (см. MathDocument.d.ts): MmlNode —
  // промежуточное дерево до типсеттинга, N — итоговый узел адаптера. Мы зовём
  // convert с дефолтным `end: STATE.LAST` (полный пайплайн, включая output jax),
  // поэтому в реализации (MathDocument.js) всегда возвращается typesetRoot, то
  // есть N. Для нашего адаптера N = LiteElement — это и есть тип, который ждёт
  // adaptor.innerHTML(node: N). Каст точный, а не `as never`/`as any`: пакет сам
  // типизирует mathjax.document() как MathDocument<any, any, any>, так что TS
  // тут промолчал бы и без каста — LiteElement делает явным то, что реально
  // проверено рантаймом.
  const raw: string = await mathjax.handleRetriesFor(() =>
    adaptor.innerHTML(doc.convert(latex, { display: true }) as LiteElement)
  )

  // Размер берём из viewBox (единица = 1/1000 em) — это единственный надёжный
  // источник. Атрибуты width/height приходят в ex, а 1ex шрифта = 0.442em, и
  // внутри <img> браузер посчитал бы ex от системного шрифта (ошибка ~13%).
  // options.exFactor (0.5) для этого тоже не годится — он про вёрстку страницы.
  const viewBox = /viewBox="([^"]*)"/.exec(raw)?.[1]
  if (!viewBox) return { svg: '', width: 0, height: 0, error: 'нет viewBox' }
  const [, , vbW, vbH] = viewBox.trim().split(/\s+/).map(Number)

  const width = +((vbW / 1000) * fontSize).toFixed(2)
  const height = +((vbH / 1000) * fontSize).toFixed(2)

  const svg = raw
    .replace(/width="[\d.]+ex"/, `width="${width}"`)
    .replace(/height="[\d.]+ex"/, `height="${height}"`)
    .replace(/ style="vertical-align:[^"]*"/, '')
    // Внутри <img> currentColor не резолвится и всё стало бы чёрным.
    // Подстановок ровно две, на узле-обёртке; глифы наследуют от него.
    .replaceAll('currentColor', color)

  // MathJax не кидает на битом вводе, а рисует merror-блок. Такой SVG зовёт
  // системный serif, то есть НЕ самодостаточен — на доску его не пускаем.
  const error = /data-mjx-error="([^"]*)"/.exec(svg)?.[1]

  return { svg, width, height, error }
}

/** base64 через UTF-8: голый btoa() падает на кириллице (InvalidCharacterError). */
export function svgToDataUrl(svg: string): string {
  const bytes = new TextEncoder().encode(svg)
  let bin = ''
  for (const b of bytes) bin += String.fromCharCode(b)
  return `data:image/svg+xml;base64,${btoa(bin)}`
}
