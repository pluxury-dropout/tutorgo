-- +goose Up
-- Уход из группы мягкий: строка записи — кусок истории участия. Фаза 2 строит
-- на ней разметку легаси-платежей и периоды участия, а DELETE стирал и то и
-- другое (docs/specs/2026-09-06-price-units-and-student-centric-money.md, п. 5a.4).
ALTER TABLE course_enrollments ADD COLUMN left_at TIMESTAMPTZ;  -- NULL = занимается

-- +goose Down
-- Прежний смысл «убран — строки нет»: иначе после отката ушедшие снова в группе.
DELETE FROM course_enrollments WHERE left_at IS NOT NULL;
ALTER TABLE course_enrollments DROP COLUMN left_at;
