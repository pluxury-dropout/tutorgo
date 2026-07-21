package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"tutorgo/models"
)

// elementSink — узкая часть WhiteboardService, нужная буферу (тот же приём, что
// у subState ниже: не тащить весь сервис ради одного метода).
type elementSink interface {
	MergeElements(ctx context.Context, pageID string, els []models.BoardElement) error
}

// flushInterval — как часто буфер уходит в БД. Секунда естественно схлопывает
// промежуточные версии растущего штриха: пока перо ведут, элемент бампает
// version десять раз в секунду, а в БД уезжает одна, последняя.
const flushInterval = time.Second

// flushThreshold — размер, при котором не ждём тика. Вставка PDF кладёт сотню
// элементов разом, и держать их лишнюю секунду незачем.
const flushThreshold = 200

// elementBuffer копит правки доски и пишет их пачками.
//
// Стоит вне горячего пути: рисование рассылается пирам сразу, а запись в
// Supabase (он в другой сети) происходит следом и никого не ждёт.
type elementBuffer struct {
	mu sync.Mutex
	// pageID → elementID → последняя известная версия элемента.
	// ponytail: без потолка на размер. Дедупликация по elementID делает буфер
	// естественно ограниченным числом элементов на активных досках — даже
	// многочасовой недоступности БД не хватит, чтобы он рос бесконечно.
	pending map[string]map[string]models.BoardElement
	wake    chan struct{}
	svc     elementSink
	log     *slog.Logger
}

func newElementBuffer(svc elementSink, log *slog.Logger) *elementBuffer {
	return &elementBuffer{
		pending: make(map[string]map[string]models.BoardElement),
		wake:    make(chan struct{}, 1),
		svc:     svc,
		log:     log,
	}
}

func (b *elementBuffer) add(pageID string, els []models.BoardElement) {
	if len(els) == 0 {
		return
	}
	b.mu.Lock()
	page := b.pending[pageID]
	if page == nil {
		page = make(map[string]models.BoardElement, len(els))
		b.pending[pageID] = page
	}
	for _, e := range els {
		// Схлопывать правки одного элемента можно только по тому же правилу,
		// что и в БД: иначе пришедшая следом СТАРАЯ версия вытеснит из буфера
		// более свежую, и та не доедет до Postgres никогда.
		if prev, ok := page[e.ID]; ok && !e.Beats(prev) {
			continue
		}
		page[e.ID] = e
	}
	big := len(page) >= flushThreshold
	b.mu.Unlock()

	if big {
		select {
		case b.wake <- struct{}{}:
		default: // флаш уже запрошен
		}
	}
}

// addPayload разбирает payload сообщения `update` и кладёт элементы в буфер.
// Мусор отбрасывается с логом, соединение не рвём: граница доверия сместилась
// вместе с ролью сервера — он перешёл из ретранслятора в хранилище.
func (b *elementBuffer) addPayload(pageID string, payload json.RawMessage) {
	var p struct {
		Elements []json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(payload, &p); err != nil || len(p.Elements) == 0 {
		return
	}
	els, err := models.ParseBoardElements(p.Elements)
	if err != nil {
		b.log.Warn("board update rejected",
			slog.String("pageId", pageID), slog.String("error", err.Error()))
		return
	}
	b.add(pageID, els)
}

func (b *elementBuffer) flush(ctx context.Context) {
	b.mu.Lock()
	pending := b.pending
	b.pending = make(map[string]map[string]models.BoardElement)
	b.mu.Unlock()

	for pageID, page := range pending {
		els := make([]models.BoardElement, 0, len(page))
		for _, e := range page {
			els = append(els, e)
		}
		if err := b.svc.MergeElements(ctx, pageID, els); err != nil {
			b.log.Error("flush board elements",
				slog.String("pageId", pageID),
				slog.Int("count", len(els)),
				slog.String("error", err.Error()))
			// Возвращаем в буфер: моргание БД должно стоить задержки, а не
			// потери рисования. add применит то же правило слияния, так что
			// правки, пришедшие за время неудачной записи, не откатятся.
			b.add(pageID, els)
		}
	}
}

// Run гоняет буфер до отмены контекста, после чего дописывает остаток.
//
// Финальный флаш идёт по СВЕЖЕМУ контексту: ctx на этот момент уже отменён, и
// запись по нему не состоялась бы. Требование «потеря нескольких секунд
// допустима» относится к сбою, не к штатному деплою — иначе каждый релиз
// откусывал бы у идущего урока последнюю секунду.
func (b *elementBuffer) Run(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.flush(ctx)
		case <-b.wake:
			b.flush(ctx)
		case <-ctx.Done():
			flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			b.flush(flushCtx)
			cancel()
			return
		}
	}
}
