// Package jobs — общие типы River-джоб: их вставляет API, исполняет воркер.
package jobs

import "github.com/riverqueue/river"

type PdfImportArgs struct {
	ImportID string `json:"import_id"`
}

func (PdfImportArgs) Kind() string { return "pdf_import" }

// 5 попыток с дефолтным backoff River (~секунды → минуты): транзиентные сбои
// S3/сети переживаем, битый файл не долбим вечно.
func (PdfImportArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5}
}

// RecurrenceExtendArgs — ночная догрузка горизонта повторений: бессрочное
// правило («спортзал каждый понедельник») материализовано на 6 месяцев вперёд,
// и кто-то должен двигать эту границу.
type RecurrenceExtendArgs struct{}

func (RecurrenceExtendArgs) Kind() string { return "recurrence_extend" }

func (RecurrenceExtendArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}
