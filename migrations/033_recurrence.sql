-- +goose Up
CREATE TABLE recurrence_rules (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id           UUID        NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    freq               TEXT        NOT NULL CHECK (freq IN ('daily','weekly','monthly')),
    interval_n         INT         NOT NULL DEFAULT 1 CHECK (interval_n > 0),
    byweekday          SMALLINT[]  NOT NULL DEFAULT '{}',   -- ISO 1=Пн … 7=Вс
    time_local         TIME        NOT NULL,
    tz                 TEXT        NOT NULL,                -- IANA, напр. 'Asia/Almaty'
    duration_minutes   INT         NOT NULL CHECK (duration_minutes > 0),
    starts_on          DATE        NOT NULL,
    ends_on            DATE,                                -- NULL = бессрочно
    max_count          INT,                                 -- NULL = без ограничения
    materialized_until DATE        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Составной, а не частичный: предикат с CURRENT_DATE Postgres не принимает
-- («functions in index predicate must be marked IMMUTABLE»), а ends_on второй
-- колонкой всё равно отсекает закончившиеся правила без обращения к таблице.
CREATE INDEX idx_rules_materialize ON recurrence_rules(materialized_until, ends_on);

-- rule_id у урока — SET NULL: правило можно удалить, а проведённые уроки с их
-- посещаемостью, досками и платежами обязаны остаться. У события удалять нечего,
-- поэтому CASCADE.
ALTER TABLE lessons
    ADD COLUMN rule_id         UUID REFERENCES recurrence_rules(id) ON DELETE SET NULL,
    ADD COLUMN occurrence_date DATE,
    ADD COLUMN is_override     BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE events
    ADD COLUMN rule_id         UUID REFERENCES recurrence_rules(id) ON DELETE CASCADE,
    ADD COLUMN occurrence_date DATE,
    ADD COLUMN is_override     BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_lessons_rule ON lessons(rule_id) WHERE rule_id IS NOT NULL;
CREATE INDEX idx_events_rule  ON events(rule_id)  WHERE rule_id IS NOT NULL;

-- Уникальность (rule_id, occurrence_date) делает материализацию идемпотентной:
-- джоба догоняет горизонт INSERT ... ON CONFLICT DO NOTHING и может падать и
-- перезапускаться сколько угодно, не плодя дублей.
CREATE UNIQUE INDEX idx_lessons_rule_occurrence ON lessons(rule_id, occurrence_date)
    WHERE rule_id IS NOT NULL;
CREATE UNIQUE INDEX idx_events_rule_occurrence ON events(rule_id, occurrence_date)
    WHERE rule_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_events_rule_occurrence;
DROP INDEX IF EXISTS idx_lessons_rule_occurrence;
DROP INDEX IF EXISTS idx_events_rule;
DROP INDEX IF EXISTS idx_lessons_rule;
ALTER TABLE events  DROP COLUMN is_override, DROP COLUMN occurrence_date, DROP COLUMN rule_id;
ALTER TABLE lessons DROP COLUMN is_override, DROP COLUMN occurrence_date, DROP COLUMN rule_id;
DROP TABLE IF EXISTS recurrence_rules;
