-- +goose Up
-- Go-модели сканируют эти колонки в обычный string, а не *string: любой NULL
-- роняет rows.Scan («cannot scan NULL into *string») и вся ручка отдаёт 500.
-- Само приложение NULL сюда не пишет — у него нулевое значение это '' — но
-- данные, залитые SQL-ом мимо Go-слоя (демо-сид для лендинга), положили NULL
-- и уронили /students, /calendar и /lessons?from=&to=.
-- Чиним инвариант там, где его можно гарантировать: в схеме.
-- materials.file_path/mime_type сюда НЕ входят — у папки NULL законен
-- (см. CHECK materials_check), и репозиторий уже читает их через COALESCE.

UPDATE lessons  SET notes      = '' WHERE notes      IS NULL;
UPDATE students SET email      = '' WHERE email      IS NULL;
UPDATE students SET notes      = '' WHERE notes      IS NULL;
UPDATE students SET last_name  = '' WHERE last_name  IS NULL;
UPDATE students SET phone      = '' WHERE phone      IS NULL;
UPDATE tutors   SET phone      = '' WHERE phone      IS NULL;

ALTER TABLE lessons  ALTER COLUMN notes     SET DEFAULT '', ALTER COLUMN notes     SET NOT NULL;
ALTER TABLE students ALTER COLUMN email     SET DEFAULT '', ALTER COLUMN email     SET NOT NULL;
ALTER TABLE students ALTER COLUMN notes     SET DEFAULT '', ALTER COLUMN notes     SET NOT NULL;
ALTER TABLE students ALTER COLUMN last_name SET DEFAULT '', ALTER COLUMN last_name SET NOT NULL;
ALTER TABLE students ALTER COLUMN phone     SET DEFAULT '', ALTER COLUMN phone     SET NOT NULL;
ALTER TABLE tutors   ALTER COLUMN phone     SET DEFAULT '', ALTER COLUMN phone     SET NOT NULL;

-- +goose Down
ALTER TABLE lessons  ALTER COLUMN notes     DROP NOT NULL, ALTER COLUMN notes     DROP DEFAULT;
ALTER TABLE students ALTER COLUMN email     DROP NOT NULL, ALTER COLUMN email     DROP DEFAULT;
ALTER TABLE students ALTER COLUMN notes     DROP NOT NULL, ALTER COLUMN notes     DROP DEFAULT;
ALTER TABLE students ALTER COLUMN last_name DROP NOT NULL, ALTER COLUMN last_name DROP DEFAULT;
ALTER TABLE students ALTER COLUMN phone     DROP NOT NULL, ALTER COLUMN phone     DROP DEFAULT;
ALTER TABLE tutors   ALTER COLUMN phone     DROP NOT NULL, ALTER COLUMN phone     DROP DEFAULT;
