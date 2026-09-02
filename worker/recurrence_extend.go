package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"tutorgo/jobs"
	"tutorgo/service"
)

// RecurrenceExtendWorker двигает горизонт материализации у активных правил.
// Идемпотентен: повторный запуск упирается в уникальный индекс
// (rule_id, occurrence_date) и не плодит дублей, поэтому ретраи безопасны.
type RecurrenceExtendWorker struct {
	river.WorkerDefaults[jobs.RecurrenceExtendArgs]

	Svc service.RecurrenceService
	Log *slog.Logger
}

func (w *RecurrenceExtendWorker) Work(ctx context.Context, job *river.Job[jobs.RecurrenceExtendArgs]) error {
	horizon := time.Now().Add(service.RecurrenceHorizon)
	n, err := w.Svc.ExtendAll(ctx, horizon)
	if err != nil {
		return err
	}
	w.Log.Info("recurrence horizon extended",
		slog.Int("created", n),
		slog.String("horizon", horizon.Format(time.DateOnly)))
	return nil
}
