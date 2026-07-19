-- +goose Up
-- Снапшот доски больше не запрашивается через SQL (приложение читает его целиком
-- в GetPageSnapshot), а jsonb за это берёт плату: парсинг при каждой записи и
-- хранение распарсенного дерева. bytea позволяет положить туда gzip.
--
-- Существующие снапшоты конвертируются в СЫРОЙ UTF-8 JSON, без сжатия: gzip в
-- Postgres нативно нет, а тащить ради разовой конвертации расширение незачем.
-- Читатель различает форматы по gzip-magic (0x1f 0x8b), так что старые записи
-- продолжают открываться и сожмутся сами при первой перезаписи.
ALTER TABLE board_pages
    ALTER COLUMN snapshot TYPE bytea USING convert_to(snapshot::text, 'UTF8');

-- +goose Down
-- Откат сработает, только пока в колонке лежит несжатый JSON. После того как
-- приложение перезапишет снапшоты gzip-ом, convert_from упрётся в бинарь —
-- тогда откат потребует распаковки на стороне приложения.
ALTER TABLE board_pages
    ALTER COLUMN snapshot TYPE jsonb USING convert_from(snapshot, 'UTF8')::jsonb;
