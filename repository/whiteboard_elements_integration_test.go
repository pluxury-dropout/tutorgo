//go:build integration

package repository_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Правило слияния живёт в двух местах: BoardElement.Beats (Go, для буфера) и
// mergeElementSQL (Postgres, авторитетно). Go-версия проверена юнит-тестами в
// models; здесь проверяется, что SQL решает ТАК ЖЕ. Расхождение этих двух даёт
// молчаливое разъезжание доски у участников — самый недебажимый класс багов в
// этой подсистеме.
//
// Отдельная переменная TEST_DB_URL, а не DB_URL: локальный .env смотрит в тот
// же Supabase, что и прод, и тест не должен иметь ни малейшего шанса туда
// дотянуться.
//
// Запуск: TEST_DB_URL=postgres://... go test -tags=integration ./repository/ -run TestBoardElements
func testPool(t *testing.T) *pgxpool.Pool {
	url := os.Getenv("TEST_DB_URL")
	if url == "" {
		t.Skip("TEST_DB_URL не задан — пропускаем integration-тест")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedPage создаёт репетитора, пробную доску и страницу, регистрирует cleanup.
func seedPage(t *testing.T, pool *pgxpool.Pool) string {
	ctx := context.Background()
	email := fmt.Sprintf("board-elements-%d@example.com", time.Now().UnixNano())

	var tutorID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor') RETURNING id`, email).Scan(&tutorID))
	// boards.tutor_id — не ON DELETE CASCADE, поэтому удаление одного репетитора
	// упирается в FK и молча не выполняется, засоряя тестовую базу. Сносим
	// сверху вниз; ошибки проверяем — тихий cleanup и был причиной мусора.
	t.Cleanup(func() {
		ctx := context.Background()
		for _, q := range []string{
			`DELETE FROM boards WHERE tutor_id = $1`,
			`DELETE FROM tutors WHERE id = $1`,
		} {
			if _, err := pool.Exec(ctx, q, tutorID); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})

	var boardID, pageID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO boards (course_id, tutor_id) VALUES (NULL, $1) RETURNING id`,
		tutorID).Scan(&boardID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO board_pages (board_id, title, position)
		 VALUES ($1, 'Страница 1', 0) RETURNING id`, boardID).Scan(&pageID))
	return pageID
}

// element несёт marker — поле, по которому видно, ЧЬЯ запись лежит в БД.
// Без него случай «version и nonce совпали» неразличим: сравнение полей не
// отличает «incoming переписал» от «base остался и он же равен incoming».
func element(id string, version int, nonce int64, marker string) models.BoardElement {
	data, _ := json.Marshal(map[string]any{
		"id": id, "version": version, "versionNonce": nonce, "marker": marker,
	})
	return models.BoardElement{ID: id, Version: version, Nonce: nonce, Data: data}
}

// stored читает, что в итоге лежит в БД.
func stored(t *testing.T, pool *pgxpool.Pool, pageID, elementID string) (int, int64, string) {
	var v int
	var n int64
	var marker string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT version, nonce, data->>'marker'
           FROM board_elements WHERE page_id = $1 AND element_id = $2`,
		pageID, elementID).Scan(&v, &n, &marker))
	return v, n, marker
}

func TestBoardElementsMergeRule(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewWhiteboardRepository(pool)
	ctx := context.Background()

	cases := []struct {
		name           string
		base, incoming models.BoardElement
		wantWin        bool // победил ли incoming
	}{
		{"version больше — принять", element("a", 1, 500, "base"), element("a", 2, 500, "in"), true},
		{"version меньше — отклонить", element("b", 2, 500, "base"), element("b", 1, 500, "in"), false},
		{"version равна, nonce меньше — принять", element("c", 2, 500, "base"), element("c", 2, 100, "in"), true},
		{"version равна, nonce больше — отклонить", element("d", 2, 500, "base"), element("d", 2, 900, "in"), false},
		{"полное совпадение — не переписывать", element("e", 2, 500, "base"), element("e", 2, 500, "in"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pageID := seedPage(t, pool)
			require.NoError(t, repo.MergeElements(ctx, pageID, []models.BoardElement{tc.base}))
			require.NoError(t, repo.MergeElements(ctx, pageID, []models.BoardElement{tc.incoming}))

			want := tc.base
			if tc.wantWin {
				want = tc.incoming
			}
			ver, nonce, marker := stored(t, pool, pageID, tc.base.ID)
			require.Equal(t, want.Version, ver, "version")
			require.Equal(t, want.Nonce, nonce, "nonce")
			// marker — единственное, что различает кандидатов при равных
			// version/nonce, и заодно проверяет, что data едет вместе с ними.
			require.Equal(t, tc.wantWin, marker == "in", "в БД лежит не та запись")

			// То же решение, что принял бы Go: две реализации правила обязаны
			// совпадать, иначе буфер и БД разойдутся.
			require.Equal(t, tc.incoming.Beats(tc.base), marker == "in",
				"SQL и BoardElement.Beats разошлись")
		})
	}
}

// Merge не удаляет то, чего нет во входящем сообщении. Это и есть структурная
// защита от инцидента 2026-07-20: пустая сцена больше не стирает доску.
func TestBoardElementsMergeNeverDeletes(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewWhiteboardRepository(pool)
	ctx := context.Background()
	pageID := seedPage(t, pool)

	require.NoError(t, repo.MergeElements(ctx, pageID, []models.BoardElement{
		element("keep", 1, 10, "base"), element("also", 1, 20, "base"),
	}))
	// Пустая сцена от клиента, не дождавшегося сида.
	require.NoError(t, repo.MergeElements(ctx, pageID, nil))

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM board_elements WHERE page_id = $1`, pageID).Scan(&count))
	require.Equal(t, 2, count, "пустая сцена стёрла доску")
}

func gz(t *testing.T, v any) []byte {
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// Импорт старого снапшота: элементы, files и флаг. Идемпотентность обязательна —
// два клиента могут открыть страницу одновременно.
func TestBoardElementsImportSnapshot(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewWhiteboardRepository(pool)
	ctx := context.Background()
	pageID := seedPage(t, pool)

	snapshot := gz(t, map[string]any{
		"elements": []map[string]any{
			{"id": "one", "version": 3, "versionNonce": 7, "index": "a1"},
			{"id": "two", "version": 1, "versionNonce": 9, "index": "a0"},
		},
		"files": map[string]any{"f1": map[string]string{"url": "/u", "mimeType": "image/png"}},
	})
	_, err := pool.Exec(ctx, `UPDATE board_pages SET snapshot = $2 WHERE id = $1`, pageID, snapshot)
	require.NoError(t, err)

	require.NoError(t, repo.ImportSnapshotToElements(ctx, pageID))
	require.NoError(t, repo.ImportSnapshotToElements(ctx, pageID)) // идемпотентно

	state, migrated, err := repo.GetPageState(ctx, pageID)
	require.NoError(t, err)
	require.True(t, migrated, "флаг elements_migrated_at не выставлен")

	var got struct {
		Elements []struct {
			ID    string `json:"id"`
			Index string `json:"index"`
		} `json:"elements"`
		Files map[string]struct {
			URL string `json:"url"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(state, &got))
	require.Len(t, got.Elements, 2)
	// Порядок — по фракционному индексу Excalidraw, это z-order сцены.
	require.Equal(t, "two", got.Elements[0].ID, "элементы не отсортированы по index")
	require.Equal(t, "one", got.Elements[1].ID)
	// files обязаны пережить переезд: без них все картинки на доске превращаются
	// в серые прямоугольники.
	require.Equal(t, "/u", got.Files["f1"].URL)
}

// Свежие tombstones держим (иначе пир, пропустивший удаление, воскресит
// элемент), старые — убираем.
func TestBoardElementsTombstoneCleanup(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewWhiteboardRepository(pool)
	ctx := context.Background()
	pageID := seedPage(t, pool)

	dead, _ := json.Marshal(map[string]any{"id": "dead", "version": 1, "versionNonce": 1, "isDeleted": true})
	fresh, _ := json.Marshal(map[string]any{"id": "fresh", "version": 1, "versionNonce": 1, "isDeleted": true})
	alive, _ := json.Marshal(map[string]any{"id": "alive", "version": 1, "versionNonce": 1})
	require.NoError(t, repo.MergeElements(ctx, pageID, []models.BoardElement{
		{ID: "dead", Version: 1, Nonce: 1, Data: dead},
		{ID: "fresh", Version: 1, Nonce: 1, Data: fresh},
		{ID: "alive", Version: 1, Nonce: 1, Data: alive},
	}))
	_, err := pool.Exec(ctx,
		`UPDATE board_elements SET updated_at = now() - interval '25 hours'
          WHERE page_id = $1 AND element_id = 'dead'`, pageID)
	require.NoError(t, err)

	_, err = repo.DeleteOldTombstones(ctx)
	require.NoError(t, err)

	var ids []string
	rows, err := pool.Query(ctx,
		`SELECT element_id FROM board_elements WHERE page_id = $1 ORDER BY element_id`, pageID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.Equal(t, []string{"alive", "fresh"}, ids)
}
