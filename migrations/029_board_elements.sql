-- +goose Up
-- Сервер становится источником истины для доски: вместо одного BLOB'а на
-- страницу — строка на элемент. Слияние выражается в WHERE-клаузе UPSERT'а
-- (см. MergeElements), поэтому клиент физически не может стереть доску,
-- прислав пустую сцену: merge не удаляет то, чего нет во входящем сообщении.
--
-- fillfactor: строки переписываются потоком (каждый штрих — UPDATE своей
-- строки). Запас в странице даёт HOT-обновления — новая версия ложится рядом
-- со старой, первичный индекс не трогается.
CREATE TABLE board_elements (
    page_id    uuid   NOT NULL REFERENCES board_pages(id) ON DELETE CASCADE,
    element_id text   NOT NULL,
    version    int    NOT NULL,
    nonce      bigint NOT NULL,
    data       jsonb  NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (page_id, element_id)
) WITH (fillfactor = 80);

-- Признак «страница уже переехала на поэлементную модель». Именно флаг, а не
-- наличие строк: через сутки чистка tombstones обнулит полностью очищенную
-- доску, и импорт по признаку «нет строк» воскресил бы старый снапшот.
ALTER TABLE board_pages ADD COLUMN elements_migrated_at timestamptz;

-- Карта fileId → {url, mimeType}: указатели на картинки в S3. Жила внутри
-- снапшота и поэлементной таблицей не покрывается — элемент image несёт
-- fileId, но не URL. Колонкой, а не таблицей: записи иммутабельны (новая
-- картинка получает новый fileId), поэтому слияние — это `files || $1`.
ALTER TABLE board_pages ADD COLUMN files jsonb NOT NULL DEFAULT '{}'::jsonb;

-- ponytail: индекса под чистку tombstones нет — на горизонте задачи таблица
-- в тысячи строк, seq scan раз в 10 минут дешевле лишнего индекса на горячих
-- UPDATE'ах. Появится боль — частичный индекс по (updated_at).

-- +goose Down
ALTER TABLE board_pages DROP COLUMN files;
ALTER TABLE board_pages DROP COLUMN elements_migrated_at;
DROP TABLE board_elements;
