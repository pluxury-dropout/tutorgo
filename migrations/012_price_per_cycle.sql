-- +goose Up
ALTER TABLE courses
  ADD COLUMN price_per_cycle   numeric(10,2),
  ADD COLUMN lessons_per_cycle int;

UPDATE courses SET price_per_cycle = price_per_lesson, lessons_per_cycle = 1;

ALTER TABLE courses
  ALTER COLUMN price_per_cycle   SET NOT NULL,
  ALTER COLUMN lessons_per_cycle SET NOT NULL;

ALTER TABLE courses
  ADD CONSTRAINT check_lessons_per_cycle_positive CHECK (lessons_per_cycle > 0);

ALTER TABLE courses DROP COLUMN price_per_lesson;

-- +goose Down
ALTER TABLE courses
  ADD COLUMN price_per_lesson numeric(10,2);

UPDATE courses SET price_per_lesson = price_per_cycle / lessons_per_cycle;

ALTER TABLE courses
  ALTER COLUMN price_per_lesson SET NOT NULL;

ALTER TABLE courses
  DROP CONSTRAINT check_lessons_per_cycle_positive,
  DROP COLUMN price_per_cycle,
  DROP COLUMN lessons_per_cycle;
