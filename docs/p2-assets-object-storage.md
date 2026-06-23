# P2 — Ассеты доски в объектное хранилище (Supabase Storage)

Рабочая заметка для продолжения позже. Контекст, решение, что сделать руками,
план реализации.

## Проблема (почему это горит)

Сейчас ассеты доски (PDF/картинки) лежат на **эфемерном диске контейнера**:

- `UploadAsset` (`handlers/whiteboard.go:152`) пишет файл в `uploads/board-assets/<uuid><ext>`.
- В БД (`board_assets.file_path`) хранится **путь на диске**.
- `ServeAsset` (`handlers/whiteboard.go:190`) отдаёт `c.File(asset.FilePath)`.

В `Dockerfile` нет `VOLUME`, директория создаётся в рантайме → **каждый редеплой/рестарт
стирает все ассеты**. `GET /public/board-assets/:id` после деплоя вернёт 404 на всё старое.
Это потеря данных уже на одном инстансе, не «риск масштабирования».

## Решение

**Путь 2 — объектное хранилище.** Выбран **Supabase Storage** (БД уже на Supabase,
ноль новой инфры, бесплатный тариф, S3-совместимое).

### Портативность на будущее (есть желание уйти на что-то быстрее)
S3 — стандарт де-факто. Пишем код против S3-API через `minio-go` один раз.
Переезд Supabase → Cloudflare R2 / MinIO / AWS S3 = поменять **endpoint + ключи в `.env`**
и **скопировать объекты**. Код не трогаем.

> ⚠️ НЕ писать свой `interface Storage` с двумя реализациями «на случай смены провайдера».
> Протокол S3 и есть слой портативности, `minio-go` — единственная нужная реализация.

## Как работает объектное хранилище (шпаргалка)

- Ментальная модель: гигантская `map[string][]byte` по HTTP. Операции: **PUT / GET / DELETE**,
  всегда целым объектом (нет дозаписи/правки середины).
- **Bucket** — пространство имён (≈ имя БД). **Key** — строка-ключ (слэши в нём косметика,
  настоящих папок нет). **Object** — байты + метаданные (content-type, size).
- Авторизация: **Access Key ID** (логин) + **Secret Access Key** (пароль), запросы
  подписываются HMAC — SDK делает сам.
- **Presigned URL** — сервер своим secret key генерит временную подписанную ссылку прямо
  на объект; браузер качает напрямую из хранилища, минуя наш сервер; ссылка протухает
  (напр. 5 мин). Аналогия — номерок из гардероба. Лучше, чем proxy: тяжёлые файлы не текут
  через Go-сервер, приватность сохраняется.

## TODO — сделать руками в дашборде Supabase (блокер, нужно до кода)

1. **Storage → Create bucket** → имя `board-assets`, тип **Private**.
2. **Storage → S3 Connection** (или Project Settings → Storage) → включить
   **S3-compatible access**, создать **S3 Access Key**. Скопировать:
   - Access Key ID
   - Secret Access Key (показывается один раз!)
3. Записать **Endpoint** (`https://<project-ref>.supabase.co/storage/v1/s3`) и **Region**.

Эти 5 значений → в `.env` (НЕ в репозиторий, как `JWT_SECRET`).

## План реализации (порядок: migration → model → repo → service → handler → router)

Безкредовые слои (можно начать сразу):
1. **migration** — переименовать `board_assets.file_path` → `object_key`.
2. **model** — `FilePath` → `ObjectKey` в `models.BoardAsset` (`models/whiteboard.go`).
3. **repo** — обновить SQL под новую колонку (`repository/whiteboard.go`).
4. **config** — каркас под 5 новых полей в `config.Load()`.

После добычи кредов:
5. **`.env`** — endpoint, region, access key, secret, bucket.
6. **`router.go`** — инициализировать `minio-go` клиент один раз, прокинуть в `WhiteboardService`
   (рядом с `pgxpool`). Новая зависимость: `github.com/minio/minio-go/v7`.
7. **`UploadAsset`** — `os.Create`+`io.Copy` → `client.PutObject(...)`; ключ `board-assets/<uuid><ext>`.
   Сохранить логику «при ошибке записи в БД удалить объект» (вместо `os.Remove` → remove из bucket).
8. **`ServeAsset`** — `c.File(path)` → сгенерировать **presigned URL** и редиректнуть (Learn-by-Doing:
   решить TTL и формат ответа).

Не трогаем: проверки владения (`verifyBoardOwnership`), лимит 20 МБ (`MaxBytesReader` + `header.Size`).

## Что дальше
Когда вернёшься: либо скажи «начинай» — сделаю безкредовые слои (1–4), либо добудь креды
и пойдём по всему плану сразу.
