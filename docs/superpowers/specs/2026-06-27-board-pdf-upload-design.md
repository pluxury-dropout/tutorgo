# Загрузка PDF на доску

## Проблема

При перетаскивании PDF на доску tldraw показывает «filetype is not allowed» — дефолтный
drag-drop tldraw принимает только изображения/видео. Учителю нужно вставлять страницы
учебников (PDF) на доску.

## Текущее состояние

Конвертация PDF→PNG **уже написана** в `frontend/src/components/whiteboard/PdfUploadToolbar.tsx`
(pdfjs-dist рендерит страницы в canvas → PNG → `whiteboardApi.uploadAsset`). Но компонент
**отвязан от UI**: его отключили при откате на дефолтный тулбар tldraw (commit `8fa2d23`),
потому что кастомная кнопка в `DefaultToolbar` ломала измерение ширины и тулбар исчезал.

Бэкенд `UploadAsset` (`handlers/whiteboard.go`) сохраняет файл по `Content-Type` без проверки
типа — PNG-заливка работает как есть. **Бэкенд менять не нужно.**

## Решения пользователя

- **Страницы:** только нужные — пользователь вводит диапазон («5-8» / «5»), не весь PDF.
- **Точка входа:** перехват drag-drop (как пробовал пользователь) + диалог диапазона.
- **Размещение:** горизонтально, встык, слева направо (как разворот учебника).
- Конвертация остаётся клиентской (pdfjs уже установлен, новых зависимостей нет).

## Архитектура

```
перетащил PDF на canvas
  → registerExternalContentHandler('files') в onMount tldraw
      ├─ не-PDF → editor.putExternalContent(default)  // отдать tldraw
      └─ application/pdf:
          → loadPdf(file) → numPages, держим pdf-объект в ref
          → открыть <PdfRangeDialog numPages=N />
          → submit(from, to):
              renderPages(pdf, from, to, onProgress) → [{blob,width,height}]
              для каждой: uploadAsset → {url,w,h}
              editor.createAssets + createShape, x = накопленная ширина (встык), y = 0
```

### Компоненты (всё на фронте)

1. **`frontend/src/lib/pdf.ts`** — вынести чистые функции из `PdfUploadToolbar.tsx`:
   - `loadPdf(file: File): Promise<PDFDocumentProxy>` — настраивает worker, возвращает doc (`.numPages`).
   - `renderPages(pdf, from, to, onProgress?): Promise<{blob: Blob; width: number; height: number}[]>`
     — рендер каждой страницы (`scale: 1.5`) в canvas → PNG blob.
   - `parseRange(input: string, numPages: number): [number, number]` — парс/валидация/clamp.

2. **`frontend/src/components/whiteboard/PdfRangeDialog.tsx`** — модалка:
   текст «Страниц: N», инпут диапазона, кнопка «Вставить», полоса прогресса конвертации.
   Props: `numPages`, `onConfirm(from, to)`, `onCancel`, `progress?`.

3. **`frontend/src/components/whiteboard/TldrawCanvas.tsx`** — в `onMount`:
   - `editor.registerExternalContentHandler('files', handler)`;
   - PDF → `loadPdf` + открыть диалог (React-стейт: `{pdf, numPages}`);
   - не-PDF → вызвать дефолтный обработчик tldraw, чтобы картинки работали как раньше;
   - после `onConfirm` — заливка и вставка встык, затем сброс стейта диалога.

4. **Удалить `frontend/src/components/whiteboard/PdfUploadToolbar.tsx`** — осиротевший,
   источник бага с исчезающим тулбаром. Логику переносим в `lib/pdf.ts`.

### Размещение встык

`x` каждой следующей страницы = сумма ширин предыдущих; `y` = 0; `w/h` = из рендера.
Вставка после точки дропа (`editor.inputs.currentPagePoint`) как базовой координаты.

## Обработка ошибок

- Битый/не читается PDF → toast «Не удалось открыть PDF», диалог не открывается.
- Отмена диалога → закрыть, ничего не вставлять.
- Диапазон вне `[1, numPages]` или перевёрнут («8-5») → clamp/swap в `parseRange`.
- Сбой `uploadAsset` на странице → прервать цикл, toast с номером страницы; уже
  вставленные страницы остаются (idempotent, без отката).

## Тестирование

Unit-тест на `parseRange` (единственная нетривиальная чистая логика):
`"5-8"`→[5,8] · `"5"`→[5,5] · `"8-5"`→[5,8] · `"0"`→[1,1] · `"3-999"`→[3,numPages] ·
`""`/мусор→[1,numPages] или ошибка валидации.

Конвертация/рендер pdfjs и интеграция с tldraw — проверяются вручную (canvas/DOM, без
смысла мокать в unit).

## Вне scope

- Серверная конвертация PDF (клиентской pdfjs достаточно для учебников).
- Распознавание текста/OCR, выбор страниц превьюшками.
- Размещение по отдельным board pages (выбрано «встык на одной»).
