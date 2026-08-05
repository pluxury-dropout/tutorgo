//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tutorgo/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DeleteStale чистит импорты старше 24ч в ЛЮБОМ статусе. Раньше условие было
// `status = 'pending'`, и оригиналы отработавших импортов (done) и битых PDF
// (failed) оставались в S3 навсегда — тест держит именно это: перечисление
// статусов, а не один happy path. Запуск: make test-integration.
func TestDeleteStaleRemovesEveryStatus(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPdfImportRepository(pool)
	ctx := context.Background()

	pageID := seedPage(t, pool)
	var boardID, tutorID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT p.board_id, b.tutor_id FROM board_pages p JOIN boards b ON b.id = p.board_id
		 WHERE p.id = $1`, pageID).Scan(&boardID, &tutorID))

	// Возраст задаём смещением от now(): запрос сравнивает с текущим временем,
	// фиксированные даты протухли бы на следующем прогоне.
	insert := func(status string, age time.Duration) string {
		key := fmt.Sprintf("board-pdf/%s-%d.pdf", status, time.Now().UnixNano())
		_, err := pool.Exec(ctx,
			`INSERT INTO board_pdf_imports (board_id, page_id, tutor_id, s3_key, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, now() - $6::interval)`,
			boardID, pageID, tutorID, key, status, fmt.Sprintf("%d minutes", int(age.Minutes())))
		require.NoError(t, err)
		return key
	}

	stale := map[string]string{}
	for _, status := range []string{"pending", "rendering", "done", "failed"} {
		stale[status] = insert(status, 25*time.Hour)
	}
	fresh := insert("done", time.Hour)

	keys, err := repo.DeleteStale(ctx)
	require.NoError(t, err)

	// Сверяемся по своим ключам, а не по длине: в тестовой базе могут лежать
	// протухшие импорты от других прогонов, и они тоже попадут в выборку.
	for status, key := range stale {
		assert.Contains(t, keys, key, "статус %s: старый импорт должен уйти в чистку", status)
	}
	assert.NotContains(t, keys, fresh, "свежий импорт трогать рано — джоба может быть ещё жива")

	var left int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM board_pdf_imports WHERE board_id = $1`, boardID).Scan(&left))
	assert.Equal(t, 1, left, "в базе должна остаться ровно свежая строка")
}
