-- +goose Up
-- Колонку опустошила миграция 036: все серии переехали на recurrence_rules, код
-- старого механизма (UpdateSeriesRequest, /lessons/series/:seriesId,
-- POST /lessons/bulk, SeriesDialog) удалён тем же коммитом.
--
-- Снимок lessons_series_backup из 036 при этом остаётся: в нём и старые
-- series_id, и обрезанные уроки. Дропать его руками, когда перенос устоится.
ALTER TABLE lessons DROP COLUMN series_id;

-- +goose Down
-- Колонка возвращается пустой; значения — из снимка 036, пока он жив.
ALTER TABLE lessons ADD COLUMN series_id UUID;

UPDATE lessons l
SET series_id = b.series_id
FROM lessons_series_backup b
WHERE b.id = l.id;
