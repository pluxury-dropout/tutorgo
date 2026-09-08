//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tutorgo/repository"
	"tutorgo/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Перенос серии на живой БД. Запуск: make test-integration.
//
// На моках эти сценарии зелёные и на сломанном коде: ранний выход Materialize
// зависит от реального materialized_until, а фикстура ставит его нулевым.

// seedSeries создаёт репетитора, курс, правило и count еженедельных уроков
// начиная с first. Каскад от tutors сносит всё остальное.
func seedSeries(t *testing.T, pool *pgxpool.Pool, first time.Time, count int) (tutorID, courseID, ruleID string) {
	ctx := context.Background()
	email := fmt.Sprintf("retime-%d@example.com", time.Now().UnixNano())

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor') RETURNING id`, email).Scan(&tutorID))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM tutors WHERE id=$1`, tutorID)
		assert.NoError(t, err)
	})

	// price_per_cycle/lessons_per_cycle — NOT NULL в схеме (миграция 012), брифу
	// они не нужны, но без них INSERT падает раньше, чем начинается сам тест.
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO courses (tutor_id, subject, started_at, price_per_cycle, lessons_per_cycle)
		 VALUES ($1, 'Английский', $2, 10000, 4) RETURNING id`, tutorID, first).Scan(&courseID))

	// ends_on обязателен: без него правило бессрочно, и Materialize в хвосте
	// Retime дольёт его вплоть до RecurrenceHorizon (6 месяцев) — assert'ы про
	// точное число вхождений иначе считали бы против плавающей величины.
	lastOccurrence := first.AddDate(0, 0, 7*(count-1))
	horizon := first.AddDate(0, 0, 7*count)
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recurrence_rules
		     (tutor_id, freq, interval_n, byweekday, time_local, tz, duration_minutes,
		      starts_on, ends_on, materialized_until)
		 VALUES ($1, 'weekly', 1, $2::smallint[], '17:00', 'Asia/Almaty', 60, $3::date, $4::date, $5::date)
		 RETURNING id`,
		tutorID, []int{int(first.Weekday()+6)%7 + 1}, first, lastOccurrence, horizon).Scan(&ruleID))

	for i := 0; i < count; i++ {
		at := first.AddDate(0, 0, 7*i)
		// $2 передан дважды под разными номерами не просто так: один параметр
		// с двумя разными приведениями (timestamptz из контекста колонки и
		// ::date) даёт Postgres 42P08 "inconsistent types deduced" — узнано на
		// этом самом тесте.
		_, err := pool.Exec(ctx,
			`INSERT INTO lessons (course_id, scheduled_at, duration_minutes, notes, status, rule_id, occurrence_date)
			 VALUES ($1, $2, 60, '', 'scheduled', $3::uuid, $4::date)`,
			courseID, at, ruleID, at)
		require.NoError(t, err)
	}
	return tutorID, courseID, ruleID
}

// «Изменить все» на пятом вхождении: первые четыре остаются на старом времени,
// с пятого по десятое встают на новое, ни одного не потеряно и не задвоено.
func TestRetime_ScopeAllKeepsEveryOccurrence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// Ближайший четверг в будущем: прошедшие уроки перенос не трогает.
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	_, courseID, ruleID := seedSeries(t, pool, first, 10)

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	fifth := first.AddDate(0, 0, 7*4)
	newStart := time.Date(fifth.Year(), fifth.Month(), fifth.Day(), 5, 0, 0, 0, time.UTC) // 10:00 Алматы

	var fifthID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, fifth).Scan(&fifthID))

	// Контракт Retime («переставляет только rule_id/occurrence_date») требует,
	// чтобы своя строка была обновлена ДО вызова — в проде это делает
	// lessonService.Update. Без этого шага пятое вхождение осталось бы на
	// старом времени, и atNewTime ниже недосчитался бы одного вхождения.
	_, err = pool.Exec(ctx, `UPDATE lessons SET scheduled_at=$2 WHERE id=$1`, fifthID, newStart)
	require.NoError(t, err)

	require.NoError(t, svc.Retime(ctx, rule, fifthID, fifth.Truncate(24*time.Hour), newStart, 60, "all"))

	var total, atNewTime int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*),
		        count(*) FILTER (WHERE (scheduled_at AT TIME ZONE 'Asia/Almaty')::time = '10:00')
		 FROM lessons WHERE course_id=$1`, courseID).Scan(&total, &atNewTime))

	assert.Equal(t, 10, total, "ни одно вхождение не потеряно и не задвоено")
	assert.Equal(t, 6, atNewTime, "с пятого по десятое — на новом времени")
}

// seedEventSeries создаёт репетитора, правило и count еженедельных событий
// начиная с first. В отличие от seedSeries события привязаны к tutor_id
// напрямую — курс им не нужен. Каскад от tutors сносит всё остальное.
func seedEventSeries(t *testing.T, pool *pgxpool.Pool, first time.Time, count int) (tutorID, ruleID string) {
	ctx := context.Background()
	email := fmt.Sprintf("retime-event-%d@example.com", time.Now().UnixNano())

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO tutors (email, password_hash, first_name, last_name)
		 VALUES ($1, 'x', 'Test', 'Tutor') RETURNING id`, email).Scan(&tutorID))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM tutors WHERE id=$1`, tutorID)
		assert.NoError(t, err)
	})

	// ends_on обязателен — та же причина, что в seedSeries: без него правило
	// бессрочно, и Materialize в хвосте Retime дольёт его вплоть до
	// RecurrenceHorizon, а точный счёт вхождений ниже станет плавающим.
	lastOccurrence := first.AddDate(0, 0, 7*(count-1))
	horizon := first.AddDate(0, 0, 7*count)
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO recurrence_rules
		     (tutor_id, freq, interval_n, byweekday, time_local, tz, duration_minutes,
		      starts_on, ends_on, materialized_until)
		 VALUES ($1, 'weekly', 1, $2::smallint[], '17:00', 'Asia/Almaty', 60, $3::date, $4::date, $5::date)
		 RETURNING id`,
		tutorID, []int{int(first.Weekday()+6)%7 + 1}, first, lastOccurrence, horizon).Scan(&ruleID))

	for i := 0; i < count; i++ {
		at := first.AddDate(0, 0, 7*i)
		_, err := pool.Exec(ctx,
			`INSERT INTO events (tutor_id, title, kind, starts_at, duration_minutes, rule_id, occurrence_date)
			 VALUES ($1, 'Спортзал', 'personal', $2, 60, $3::uuid, $4::date)`,
			tutorID, at, ruleID, at)
		require.NoError(t, err)
	}
	return tutorID, ruleID
}

// «Изменить все» на серии СОБЫТИЙ (спека §6.2.2, п.2): пятое вхождение — якорь,
// первые четыре остаются на старом времени, с пятого по десятое встают на
// новое. Дефект A прямо сейчас портит данные именно у событий: events-половина
// DeleteFutureByRule (другой предикат — NOT cancelled вместо status='scheduled'),
// ReassignToRule и JOIN LATERAL в InsertOccurrences нигде в репозитории не
// выполняются на живой БД, кроме этого теста — на моках «шаблон не нашёлся»
// подделать нечем.
func TestRetime_ScopeAllKeepsEveryEventOccurrence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, ruleID := seedEventSeries(t, pool, first, 10)

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	fifth := first.AddDate(0, 0, 7*4)
	newStart := time.Date(fifth.Year(), fifth.Month(), fifth.Day(), 5, 0, 0, 0, time.UTC) // 10:00 Алматы

	var fifthID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM events WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, fifth).Scan(&fifthID))

	// Контракт Retime («переставляет только rule_id/occurrence_date») требует,
	// чтобы своя строка была обновлена ДО вызова — в проде это делает
	// eventService.Update.
	_, err = pool.Exec(ctx, `UPDATE events SET starts_at=$2 WHERE id=$1`, fifthID, newStart)
	require.NoError(t, err)

	require.NoError(t, svc.Retime(ctx, rule, fifthID, fifth.Truncate(24*time.Hour), newStart, 60, "all"))

	var total, atNewTime int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*),
		        count(*) FILTER (WHERE (starts_at AT TIME ZONE 'Asia/Almaty')::time = '10:00')
		 FROM events WHERE tutor_id=$1`, tutorID).Scan(&total, &atNewTime))

	assert.Equal(t, 10, total, "ни одно вхождение не потеряно и не задвоено")
	assert.Equal(t, 6, atNewTime, "с пятого по десятое — на новом времени")
}

// Перенос серии со своего дня недели на другой: byweekday правила меняется,
// все будущие вхождения встают на новый день, прошедшие остаются на старом.
func TestRetime_ScopeAllMovesWeekday(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	_, courseID, ruleID := seedSeries(t, pool, first, 6)

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	second := first.AddDate(0, 0, 7)
	// Суббота той же недели.
	newStart := time.Date(second.Year(), second.Month(), second.Day(), 5, 0, 0, 0, time.UTC).AddDate(0, 0, 2)

	var secondID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, second).Scan(&secondID))

	// Контракт Retime («переставляет только rule_id/occurrence_date») требует
	// обновить свою строку ДО вызова — в проде это делает lessonService.Update.
	_, err = pool.Exec(ctx, `UPDATE lessons SET scheduled_at=$2 WHERE id=$1`, secondID, newStart)
	require.NoError(t, err)

	require.NoError(t, svc.Retime(ctx, rule, secondID, second.Truncate(24*time.Hour), newStart, 60, "all"))

	var byweekday []int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT byweekday FROM recurrence_rules WHERE id=$1`, ruleID).Scan(&byweekday))
	assert.Equal(t, []int{6}, byweekday, "правило переехало на субботу")

	var total, onSaturday, onThursday int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*),
		        count(*) FILTER (WHERE EXTRACT(ISODOW FROM occurrence_date) = 6),
		        count(*) FILTER (WHERE EXTRACT(ISODOW FROM occurrence_date) = 4)
		 FROM lessons WHERE course_id=$1`, courseID).Scan(&total, &onSaturday, &onThursday))
	// Якорь (реассайнутая вторая) плюс три добитых субботы до ends_on:
	// ends_on правило не двигает, только time_local/byweekday, поэтому
	// материализация останавливается там же, где остановилась бы старая серия.
	assert.Equal(t, 5, total, "первая осталась, вторая переехала, ends_on добил ещё три субботы")
	assert.Equal(t, 4, onSaturday, "якорь плюс три добитых субботы")
	assert.Equal(t, 1, onThursday, "первое вхождение — в прошлом относительно cut, осталось на четверге")
}

// Вручную перенесённое вхождение переживает «изменить все»: is_override —
// единственное, что отделяет осознанный перенос от строки, которую можно снести
// и пересоздать по расписанию.
func TestRetime_ScopeAllSparesOverriddenOccurrence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	_, courseID, ruleID := seedSeries(t, pool, first, 6)

	// Третье вхождение репетитор подвинул руками на день позже.
	third := first.AddDate(0, 0, 14)
	var thirdID string
	require.NoError(t, pool.QueryRow(ctx,
		`UPDATE lessons SET is_override = TRUE, scheduled_at = scheduled_at + interval '1 day'
		 WHERE rule_id=$1::uuid AND occurrence_date=$2::date RETURNING id`,
		ruleID, third).Scan(&thirdID))

	repo := repository.NewRecurrenceRepository(pool)
	svc := service.NewRecurrenceService(repo)
	rule, err := repo.GetByID(ctx, ruleID)
	require.NoError(t, err)

	newStart := time.Date(first.Year(), first.Month(), first.Day(), 5, 0, 0, 0, time.UTC)
	var firstID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, first).Scan(&firstID))

	// Контракт Retime («переставляет только rule_id/occurrence_date») требует
	// обновить свою строку ДО вызова — в проде это делает lessonService.Update.
	_, err = pool.Exec(ctx, `UPDATE lessons SET scheduled_at=$2 WHERE id=$1`, firstID, newStart)
	require.NoError(t, err)

	require.NoError(t, svc.Retime(ctx, rule, firstID, first.Truncate(24*time.Hour), newStart, 60, "all"))

	var stillOverridden bool
	var localTime string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT is_override, (scheduled_at AT TIME ZONE 'Asia/Almaty')::time::text
		 FROM lessons WHERE id=$1`, thirdID).Scan(&stillOverridden, &localTime))
	assert.True(t, stillOverridden, "перенос пережил правку всей серии")
	assert.Equal(t, "17:00:00", localTime, "и остался на своём времени, а не на новом")

	var total int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1`, courseID).Scan(&total))
	assert.Equal(t, 6, total, "пересоздание не задвоило перенесённое вхождение")
}

// «Это и все следующие» на удаление: текущее вхождение отменяется (строка
// остаётся тумбстоном — удали её, и материализация вернёт урок на свободную
// дату), последующие удаляются, правило закрывается, джоба их не возвращает.
func TestDeleteFollowing_CancelsCurrentAndClosesRule(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := nextWeekday(time.Now().AddDate(0, 0, 1), time.Thursday).
		Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 8)

	third := first.AddDate(0, 0, 14)
	var thirdID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM lessons WHERE rule_id=$1::uuid AND occurrence_date=$2::date`,
		ruleID, third).Scan(&thirdID))

	svc := service.NewLessonService(repository.NewLessonRepository(pool),
		repository.NewCourseRepository(pool), repository.NewPaymentRepository(pool),
		service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)))

	require.NoError(t, svc.Delete(ctx, thirdID, tutorID, "following"))

	var status string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM lessons WHERE id=$1`, thirdID).Scan(&status))
	assert.Equal(t, "cancelled", status, "текущее вхождение отменено, а не удалено")

	var later int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1 AND occurrence_date > $2::date`,
		courseID, third).Scan(&later))
	assert.Zero(t, later, "последующие удалены")

	// seedSeries ставит materialized_until на неделю позже ends_on (нужно
	// другим трём тестам), поэтому Occurrences обрывает скан по ends_on раньше,
	// чем доходит до watermark, — ExtendAll вернул бы пусто для ЛЮБОГО правила,
	// не только закрытого. Форсируем watermark в точку до ends_on, чтобы
	// проверка что-то доказывала: незакрытое правило тут действительно
	// материализовало бы удалённые вхождения обратно.
	_, err := pool.Exec(ctx,
		`UPDATE recurrence_rules SET materialized_until = $2::date WHERE id = $1`,
		ruleID, third)
	require.NoError(t, err)

	rec := service.NewRecurrenceService(repository.NewRecurrenceRepository(pool))
	_, err = rec.ExtendAll(ctx, time.Now().Add(service.RecurrenceHorizon))
	require.NoError(t, err)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1 AND occurrence_date > $2::date`,
		courseID, third).Scan(&later))
	assert.Zero(t, later, "джоба не вернула удалённое: правило закрыто")
}

// Архивация курса: будущие уроки уходят, завершённые остаются, а ночная джоба
// не возвращает удалённое — правило закрыто и выпало из DueForMaterialization.
func TestArchiveCourseSchedule_JobDoesNotBringLessonsBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// Первое вхождение в прошлом: часть уроков успела стать completed.
	first := time.Now().AddDate(0, 0, -21).Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 10)

	_, err := pool.Exec(ctx,
		`UPDATE lessons SET status='completed' WHERE course_id=$1 AND scheduled_at < NOW()`, courseID)
	require.NoError(t, err)

	lessonRepo := repository.NewLessonRepository(pool)
	svc := service.NewLessonService(lessonRepo, repository.NewCourseRepository(pool),
		repository.NewPaymentRepository(pool),
		service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)))

	require.NoError(t, svc.ArchiveCourseSchedule(ctx, courseID, tutorID))

	var future, past int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE scheduled_at > NOW()),
		        count(*) FILTER (WHERE scheduled_at < NOW())
		 FROM lessons WHERE course_id=$1`, courseID).Scan(&future, &past))
	assert.Zero(t, future, "будущих уроков не осталось")
	// Точный счёт был бы флаки: first = now−21d усечён до суток и сдвинут на
	// +12ч, поэтому урок дня 0 попадает в прошлое только если тест бежит после
	// полудня UTC — ожидаемое значение 3 или 4 в зависимости от времени запуска.
	assert.GreaterOrEqual(t, past, 3, "завершённые остались")

	// seedSeries ставит materialized_until на неделю позже ends_on (нужно
	// другим трём тестам), поэтому без форсирования Occurrences уже обрывает
	// скан по НЕзакрытому ends_on (today+42 в этом тесте) раньше, чем доходит
	// до watermark (today+49) — ExtendAll вернул бы пусто для ЛЮБОГО правила,
	// закрытого CloseRule или нет. Форсируем watermark в прошлое (first), чтобы
	// проверка что-то доказывала: незакрытое правило дотянулось бы Occurrences
	// от такого watermark сквозь текущий день и материализовало бы будущие
	// вхождения обратно; закрытое же вообще не попадёт в выборку
	// DueForMaterialization — та фильтрует по ends_on > CURRENT_DATE, а
	// CloseRule ставит ends_on на вчера.
	_, err = pool.Exec(ctx,
		`UPDATE recurrence_rules SET materialized_until = $2::date WHERE id = $1`,
		ruleID, first)
	require.NoError(t, err)

	// Ночная джоба: правило закрыто, возвращать ей нечего.
	rec := service.NewRecurrenceService(repository.NewRecurrenceRepository(pool))
	_, err = rec.ExtendAll(ctx, time.Now().Add(service.RecurrenceHorizon))
	require.NoError(t, err)

	var afterJob int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id=$1 AND scheduled_at > NOW()`,
		courseID).Scan(&afterJob))
	assert.Zero(t, afterJob, "джоба не вернула уроки архивированного курса")

	var endsOn *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT ends_on FROM recurrence_rules WHERE id=$1`, ruleID).Scan(&endsOn))
	require.NotNil(t, endsOn, "правило закрыто, а не оставлено бессрочным")
}

// «Удалить все уроки» не оставляет правил-сирот.
func TestDeleteByCourse_LeavesNoOrphanRules(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	first := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour).Add(12 * time.Hour)
	tutorID, courseID, ruleID := seedSeries(t, pool, first, 5)

	svc := service.NewLessonService(repository.NewLessonRepository(pool),
		repository.NewCourseRepository(pool), repository.NewPaymentRepository(pool),
		service.NewRecurrenceService(repository.NewRecurrenceRepository(pool)))

	require.NoError(t, svc.DeleteByCourse(ctx, courseID, tutorID))

	var rules int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM recurrence_rules WHERE id=$1`, ruleID).Scan(&rules))
	assert.Zero(t, rules, "правило удалено вместе с уроками")
}

// nextWeekday — ближайший день недели wd не раньше from.
func nextWeekday(from time.Time, wd time.Weekday) time.Time {
	for d := 0; d < 7; d++ {
		if day := from.AddDate(0, 0, d); day.Weekday() == wd {
			return day
		}
	}
	return from
}
