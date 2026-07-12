-- +goose Up
-- Пустая строка phone='' попадала в частичный unique-индекс (в отличие от NULL),
-- поэтому второй активируемый ученик без телефона падал с 23505. Исключаем ''.
DROP INDEX IF EXISTS students_phone_account_key;
CREATE UNIQUE INDEX students_phone_account_key
  ON students (phone) WHERE password_hash IS NOT NULL AND phone <> '';

-- +goose Down
DROP INDEX IF EXISTS students_phone_account_key;
CREATE UNIQUE INDEX students_phone_account_key
  ON students (phone) WHERE password_hash IS NOT NULL;
