package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tutorgo/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

// wbMaxMessageBytes — предел на одно входящее WS-сообщение от клиента.
// Используется дважды: в SetReadLimit (байты на проводе) и как cap для
// io.LimitReader поверх распакованного потока в readPump.
//
// Раньше одного SetReadLimit хватало: без сжатия "на проводе" и "в памяти
// после чтения" — одно и то же число. С permessage-deflate это перестало быть
// так: SetReadLimit по-прежнему считает только сжатые байты кадра (gorilla
// сравнивает readLimit с длиной, заявленной в заголовке фрейма, — это
// проверяется ДО распаковки), а сам разжатый поток отдаётся через
// io.ReadAll(NextReader()) без какой-либо верхней границы. При коэффициенте
// сжатия raw-deflate до ~1000:1 на сильно повторяющихся данных (тот же класс
// проблемы, что decompression bomb, CWE-409) фрейм в 512 КБ на проводе может
// распаковаться в сотни МБ в процессе, который держит в памяти хабы ВСЕХ
// досок разом — не только текущей. Поэтому предел применяется вручную ещё
// раз, уже к распакованным байтам (см. readPump). Не убирать этот второй
// лимит при рефакторинге: без него защита от SetReadLimit — иллюзия.
const wbMaxMessageBytes = 512 * 1024

// wbCloseWriteWait — дедлайн на отправку close-фрейма при принудительном
// разрыве. Равен writeWait, который сама gorilla использует для ровно той же
// ситуации (conn.go:935, превышение SetReadLimit) — не выдумываем свой
// таймаут для симметричного случая. На пути атаки (гость шлёт мусор, ждать
// его нечего) держать горутину дольше секунды нет смысла.
const wbCloseWriteWait = 1 * time.Second

// wbUpdateThrottleMs зеркалит UPDATE_THROTTLE_MS из
// frontend/src/components/whiteboard/useExcalidrawSync.ts:49 — клиент шлёт
// update (и viewport — тот же троттл) не чаще, чем раз в это число
// миллисекунд (`if (!updateTimerRef.current)` в flushUpdate ограничивает
// именно частоту флашей, не их размер). Го-константа НЕ импортируется из
// фронта — единственное место, где связь может тихо порваться при будущей
// правке троттла. Поменяли одно — проверьте другое.
const wbUpdateThrottleMs = 100

// wbByteRateLimit/wbByteRateBurst — потолок на РАСПАКОВАННЫЙ поток ОДНОГО
// соединения, байт/сек (token bucket, golang.org/x/time/rate — как в
// middleware/rate_limit.go). wbMaxMessageBytes закрывает бомбу в одном
// сообщении, но сжатие обрушило именно СТОИМОСТЬ ПОТОКА бомб: на канале
// 10 Мбит/с (~1.25 МБ/с сырых байт) кадр в ~600 Б на проводе (тот же
// коэффициент ~1000:1, что и в комментарии у wbMaxMessageBytes) даёт
// ~2000 кадров/с, и каждый распаковывается в 512 КБ — порядка 1 ГБ/с
// аллокаций (io.ReadAll + json.Unmarshal + фан-аут по пирам + buf.addPayload)
// в процессе, который держит хабы ВСЕХ досок. Лимит по числу сообщений тут не
// годится: сообщения разного размера, счётчик либо душит рисование, либо не
// остановит поток мелких бомб — лимитировать нужно именно байты.
//
// Лимит ОБЯЗАН считаться от wbMaxMessageBytes, а не от типичного размера
// кадра. Первая версия была выведена из "типичного" freedraw-штриха
// (~100 КБ) и разошлась с тем, что на самом деле разрешает гард размера:
// система уже сегодня допускает wbMaxMessageBytes КАЖДЫЕ wbUpdateThrottleMs
// мс — например, драг крупного выделения гоняет flushUpdate с диффом ВСЕХ
// выделенных элементов разом, и 4-5 штрихов в выделении легко подводят кадр
// к 512 КБ. Гард размера такой кадр пропускает; лимитер, посчитанный от
// "типичного" случая, рвал ровно такое соединение посреди жеста (эмпирически
// подтверждено: 500 КБ раз в 100 мс убивало соединение за ~0.91с при
// лимите 4 МБ/с). Формула ниже делает такое расхождение невозможным: правка
// wbMaxMessageBytes или wbUpdateThrottleMs тянет лимит за собой.
//
//	wbByteRateLimit = wbMaxMessageBytes × (1000 / wbUpdateThrottleMs) × запас
//
// (1000 / wbUpdateThrottleMs) — максимум флашей в секунду, которые троттл
// физически допускает. Запас ×2 — не защита от легитимного пика (потолок
// точный, не оценка), а буфер на джиттер сети/шедулера, из-за которого два
// кадра-максимума могут прийти теснее номинального троттла. Итог — порядка
// 10 МБ/с: атака всё ещё падает на два порядка (с ~1 ГБ/с до ~10 МБ/с,
// стократное сокращение) — это меньше, чем дал бы атакующему гигабитный
// аплинк вообще без сжатия. Burst — 2×wbMaxMessageBytes: гарантирует, что
// одно сообщение на пределе wbMaxMessageBytes всегда проходит (AllowN(n) с
// n > burst не проходит никогда — это не запас, а необходимое условие
// корректности), и оставляет место ещё под одно такое же следом (update и
// viewport в одном тике).
const (
	wbByteRateLimit = wbMaxMessageBytes * (1000 / wbUpdateThrottleMs) * 2
	wbByteRateBurst = 2 * wbMaxMessageBytes
)

// WbMsg — message between client and hub
type WbMsg struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	X       float64         `json:"x,omitempty"`
	Y       float64         `json:"y,omitempty"`
	PeerID  string          `json:"peerId,omitempty"`
	Name    string          `json:"name,omitempty"` // отображаемое имя, клиент шлёт с cursor
	// UID — стабильный id человека (tutorID/studentID), клиент шлёт с cursor.
	// Нужен, чтобы два соединения одного человека (вкладка звонка + вкладка
	// доски) схлопывались в одного участника.
	// ponytail: значение клиентское и не проверяется — как и Name. Подмена даёт
	// только слипание аватарок, доступ к доске она не расширяет (он уже выдан
	// invite-токеном). Если понадобится доверенный id — брать его из JWT в
	// ServeWS, но у гостя JWT нет по дизайну.
	UID string `json:"uid,omitempty"`
}

// wbClient — one WebSocket connection
type wbClient struct {
	hub    *wbHub
	conn   *websocket.Conn
	send   chan []byte
	peerID string
	// limiter — токен-бакет на распакованные байты входящего потока этого
	// соединения. См. wbByteRateLimit/wbByteRateBurst: закрывает DoS через
	// поток мелких сжатых кадров, который wbMaxMessageBytes (лимит на ОДНО
	// сообщение) не видит.
	limiter *rate.Limiter
}

// wbHub — coordinator for one board page.
// Состояния доски хаб в памяти не держит: правки он рассылает пирам и отдаёт в
// elementBuffer, который пишет их в board_elements пачками. Сид нового клиента
// собирает ServeWS из той же таблицы.
type wbHub struct {
	pageID     string
	clients    map[*wbClient]bool
	broadcast  chan wbBroadcast
	register   chan *wbClient
	unregister chan *wbClient
	done       chan struct{}
	mgr        *WbHubManager
	log        *slog.Logger
}

type wbBroadcast struct {
	sender *wbClient
	data   []byte
}

func newWbHub(pageID string, mgr *WbHubManager) *wbHub {
	return &wbHub{
		pageID:     pageID,
		clients:    make(map[*wbClient]bool),
		broadcast:  make(chan wbBroadcast, 64),
		register:   make(chan *wbClient),
		unregister: make(chan *wbClient),
		done:       make(chan struct{}),
		mgr:        mgr,
		log:        mgr.log,
	}
}

func (h *wbHub) run() {
	reg := newPresenceRegistry()
	// Коалесцируем эфемерные сообщения: свежий кадр вытесняет старый, рассылаем
	// пачкой раз в ~33мс — курсоры всем, вьюпорт только подписчикам.
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	send := func(data []byte, to *wbClient) {
		select {
		case to.send <- data:
		default:
			delete(h.clients, to)
			close(to.send)
		}
	}
	// ponytail: линейный поиск клиента по peerID. Клиентов на страницу единицы
	// (препод+ученик), карта byPeer себя не окупает; ввести, если N вырастет.
	clientByPeer := func(peerID string) *wbClient {
		for c := range h.clients {
			if c.peerID == peerID {
				return c
			}
		}
		return nil
	}

	// broadcastFollowers рассылает «за кем сколько следят». Каждому свой кадр:
	// собственный peerID клиент не знает (сервер его не сообщает), поэтому его
	// счётчик кладём отдельным полем me. Эфемерка, как курсоры: потерялось —
	// перерисуется на следующем follow.
	broadcastFollowers := func() {
		counts := reg.followerCounts()
		for c := range h.clients {
			payload, _ := json.Marshal(struct {
				Counts map[string]int `json:"counts"`
				Me     int            `json:"me"`
			}{counts, counts[c.peerID]})
			data, _ := json.Marshal(WbMsg{Type: "followers", Payload: payload})
			send(data, c)
		}
	}

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// Сидинг делает ServeWS: он читает снапшот из БД до апгрейда и
			// кладёт его в client.send сам. Хабу состояние знать не нужно.
			// Новичку — текущая картина слежки. Пока никто ни за кем не
			// следит, рассказывать нечего: не шлём пустой кадр на каждый
			// коннект (у клиента и так дефолт «счётчиков нет»).
			if len(reg.following) > 0 {
				broadcastFollowers()
			}

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				hadFollow := len(reg.following) > 0
				reg.remove(client.peerID)
				close(client.send)
				// Уведомляем оставшихся, чтобы они убрали этого пира из списка
				// коллабораторов (иначе счётчик участников завышен). Best-effort:
				// на переполненный буфер пропускаем — аватар догниёт до reload.
				leave, _ := json.Marshal(WbMsg{Type: "leave", PeerID: client.peerID})
				for c := range h.clients {
					select {
					case c.send <- leave:
					default:
					}
				}
				// Ушедший мог быть и подписчиком, и целью — счётчики у
				// оставшихся протухли. Проверяем ДО remove: после него следов
				// его связей уже нет.
				if hadFollow {
					broadcastFollowers()
				}
			}
			if len(h.clients) == 0 {
				// Last client left: tear down the hub. Take the manager lock and
				// re-verify emptiness, then remove ourselves and signal done so
				// any in-flight registration retries with a fresh hub.
				h.mgr.mu.Lock()
				if len(h.clients) == 0 {
					delete(h.mgr.hubs, h.pageID)
					close(h.done)
					h.mgr.mu.Unlock()
					return
				}
				h.mgr.mu.Unlock()
			}

		case <-ticker.C:
			for _, o := range reg.flush() {
				if o.toAll {
					for c := range h.clients {
						if c.peerID == o.origin {
							continue
						}
						send(o.data, c)
					}
					continue
				}
				for _, pid := range o.to {
					if c := clientByPeer(pid); c != nil {
						send(o.data, c)
					}
				}
			}

		case msg := <-h.broadcast:
			var parsed WbMsg
			if err := json.Unmarshal(msg.data, &parsed); err != nil {
				continue
			}

			// cursor/viewport/follow разыменовывают msg.sender. Серверные пуши
			// (sender=nil) такие типы не шлют, но гардим — иначе будущий такой
			// пуш уронил бы паникой всю run()-горутину хаба без recover.
			if msg.sender == nil &&
				(parsed.Type == "cursor" || parsed.Type == "viewport" || parsed.Type == "follow") {
				continue
			}

			switch parsed.Type {
			case "cursor":
				// Штампуем peerId и кладём в presence; рассылку делает тикер.
				parsed.PeerID = msg.sender.peerID
				stamped, _ := json.Marshal(parsed)
				reg.setCursor(msg.sender.peerID, stamped)
				continue

			case "viewport":
				parsed.PeerID = msg.sender.peerID
				stamped, _ := json.Marshal(parsed)
				reg.setViewport(msg.sender.peerID, stamped)
				continue

			case "follow":
				// Подписка/отписка. При FOLLOW сразу юникастим текущий вьюпорт
				// цели — камера ведомого снапится, не дожидаясь движения ведущего.
				var f struct {
					Target string `json:"target"`
					Action string `json:"action"`
				}
				_ = json.Unmarshal(parsed.Payload, &f)
				if f.Action == "UNFOLLOW" {
					reg.unfollow(msg.sender.peerID)
				} else if snap := reg.follow(msg.sender.peerID, f.Target); snap != nil {
					send(snap, msg.sender)
				}
				broadcastFollowers()
				continue

			case "followers":
				// Тип серверный: счётчики слежки хаб считает сам. Пришедший от
				// клиента кадр — только попытка подрисовать соседям чужие
				// цифры, поэтому в default-ретрансляцию его не пускаем.
				continue

			case "update":
				// Ретранслируем как раньше — но теперь ещё и пишем: сервер стал
				// источником истины. Сама запись отложена до конца цикла (см.
				// ниже): разбор JSON не должен стоять между пиром и его штрихом.

			default:
				// Unknown message type: relay verbatim.
			}

			// Broadcast to all except sender.
			for client := range h.clients {
				if client == msg.sender {
					continue
				}
				select {
				case client.send <- msg.data:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}

			// Персист — строго после рассылки: рисование не ждёт ни разбора
			// JSON, ни тем более Supabase (буфер пишет в БД своей горутиной).
			//
			// Пишем и серверные пуши (sender == nil): элементы, которые
			// сгенерировал воркер импорта PDF, — такое же содержимое доски.
			if parsed.Type == "update" {
				h.mgr.buf.addPayload(h.pageID, parsed.Payload)
			}
		}
	}
}

func (c *wbClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(wbMaxMessageBytes)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, r, err := c.conn.NextReader()
		if err != nil {
			break
		}
		// ReadMessage() внутри и есть NextReader()+io.ReadAll() (conn.go:1102-
		// 1110) — разворачиваем его руками, чтобы вставить LimitReader между
		// ними. См. wbMaxMessageBytes: при включённом сжатии SetReadLimit выше
		// бьёт только по проводу, а без этого LimitReader распакованный поток
		// ничем не ограничен. +1, чтобы отличить "ровно лимит" от "больше
		// лимита" одним чтением.
		data, err := io.ReadAll(io.LimitReader(r, wbMaxMessageBytes+1))
		if err != nil {
			break
		}
		if len(data) > wbMaxMessageBytes {
			// Не молчим: обрыв без следа неотличим от обычного дисконнекта, а
			// это как раз тот случай, который стоит увидеть в логах — либо
			// баг клиента, либо decompression bomb (CWE-409).
			c.hub.log.Warn("board ws: decompressed message over limit, closing",
				slog.String("pageId", c.hub.pageID), slog.String("peerId", c.peerID))
			_ = c.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseMessageTooBig, ""),
				time.Now().Add(wbCloseWriteWait))
			break
		}
		// Гард выше ловит бомбу в ОДНОМ сообщении; этот — поток из МНОГИХ
		// небольших сообщений, каждое честного размера, но суммарно
		// разжимающих на сервере на порядки больше, чем клиент реально отправил
		// на проводе. См. wbByteRateLimit — легитимное рисование в этот
		// бюджет укладывается с многократным запасом, упор в лимит — сигнал,
		// не штатная ситуация.
		if !c.limiter.AllowN(time.Now(), len(data)) {
			c.hub.log.Warn("board ws: byte-rate limit exceeded, closing",
				slog.String("pageId", c.hub.pageID), slog.String("peerId", c.peerID))
			_ = c.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, ""),
				time.Now().Add(wbCloseWriteWait))
			break
		}
		c.hub.broadcast <- wbBroadcast{sender: c, data: data}
	}
}

func (c *wbClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WbHubManager — stores active hubs (one per pageID)
// subState — узкий интерфейс подписки, чтобы WS-гейт не тянул весь сервис
// (как middleware.SubscriptionState). Даёт compute-on-read состояние по tutorID.
type subState interface {
	State(ctx context.Context, tutorID string) (string, error)
}

type WbHubManager struct {
	mu        sync.Mutex
	hubs      map[string]*wbHub
	svc       service.WhiteboardService
	subs      subState
	log       *slog.Logger
	jwtSecret string
	origins   map[string]bool
	upgrader  websocket.Upgrader
	buf       *elementBuffer
}

func NewWbHubManager(svc service.WhiteboardService, subs subState, log *slog.Logger, jwtSecret string, allowedOrigins []string) *WbHubManager {
	origins := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		// Match gin-cors's tolerance: env vars often carry stray whitespace or a
		// trailing slash, which gin-cors trims but a raw map lookup would not —
		// that mismatch silently 403s the WS upgrade while REST CORS still works.
		if o = normalizeOrigin(o); o != "" {
			origins[o] = true
		}
	}
	m := &WbHubManager{
		hubs:      make(map[string]*wbHub),
		svc:       svc,
		subs:      subs,
		log:       log,
		jwtSecret: jwtSecret,
		origins:   origins,
		buf:       newElementBuffer(svc, log),
	}
	m.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     m.checkOrigin,
		// Штрихи — это массивы из сотен близких float64-координат: жмутся в
		// разы даже без общего словаря между сообщениями (permessage-deflate
		// в gorilla поддерживает только no_context_takeover, RFC 7692).
		// Уровень сжатия не задаём: дефолт gorilla — flate.BestSpeed (1), то
		// же обоснование, что у gzip.BestSpeed в repository/whiteboard.go —
		// данные сжимаются десятки раз в секунду, максимальный уровень даёт
		// проценты ценой CPU. Опасность decompression bomb и DoS потоком
		// бомб, которую даёт сжатие без ограничений, закрыта отдельно в
		// readPump — см. wbMaxMessageBytes и wbByteRateLimit.
		//
		// Побочный эффект шире, чем только штрихи: как только сжатие
		// негоциировано, WriteMessage теряет zero-alloc fast path на ВСЕХ
		// исходящих кадрах этого соединения, а не только текстовых с
		// данными доски. Fast path в gorilla включён, пока
		// c.newCompressionWriter == nil (conn.go:769); после негоциации он
		// не nil на весь срок жизни соединения — даже 30-секундный ping из
		// writePump идёт через общий (аллоцирующий) NextWriter-путь, хотя
		// сам ping не сжимается (isData(Ping) == false, conn.go:533,747).
		// Корректность не страдает, но профиль аллокаций меняется для
		// соединения целиком, а не только для payload'ов доски.
		EnableCompression: true,
	}
	return m
}

// checkOrigin mirrors the router's CORS allowlist to prevent cross-site
// WebSocket hijacking. Requests without an Origin header (e.g. non-browser
// clients) are allowed.
func (m *WbHubManager) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return m.origins[normalizeOrigin(origin)]
}

// normalizeOrigin mirrors gin-cors's origin handling (trim + lowercase) and also
// drops a trailing slash, so the WS allowlist and the REST CORS allowlist accept
// exactly the same set of origins.
func normalizeOrigin(o string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(o)), "/")
}

func (m *WbHubManager) getOrCreate(pageID string) *wbHub {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hub, ok := m.hubs[pageID]; ok {
		return hub
	}
	hub := newWbHub(pageID, m)
	m.hubs[pageID] = hub
	go hub.run()
	return hub
}

// PushToPage вбрасывает серверное сообщение в хаб страницы. Если хаба нет —
// никто не подключён, и слать некому: молча выходим (клиент увидит контент из
// снапшота/S3 при следующем подключении). sender=nil ⇒ run() раздаст всем.
// Предназначено для типов, которые run() ретранслирует вербатим (default-ветка);
// эфемерные cursor/viewport/follow с sender=nil run() безопасно отбрасывает.
func (m *WbHubManager) PushToPage(pageID string, data []byte) {
	m.mu.Lock()
	hub, ok := m.hubs[pageID]
	m.mu.Unlock()
	if !ok {
		return
	}
	select {
	case hub.broadcast <- wbBroadcast{sender: nil, data: data}:
	default:
		// Переполненный канал — не повод блокировать слушателя NOTIFY.
		m.log.Warn("board hub broadcast full, drop server event", slog.String("pageId", pageID))
	}
}

// Authorize validates the ?token= query param. It first tries to parse it as a
// tutor access JWT (same scheme as middleware.Auth); on success it requires that
// pageID's board belongs to the tutor. Otherwise it treats the token as an invite
// UUID and requires that pageID belongs to the invite's board. Returns true iff
// the client may access the page.
func (m *WbHubManager) Authorize(ctx context.Context, svc service.WhiteboardService, pageID, token string) bool {
	if token == "" {
		return false
	}

	// Try tutor access JWT first.
	if tutorID, ok := m.parseTutorJWT(token); ok {
		owned, err := svc.PageBelongsToTutor(ctx, pageID, tutorID)
		if err != nil || !owned {
			return false
		}

		state, _ := m.subs.State(ctx, tutorID)
		switch state {
		case service.StateActive, service.StateGrace:
			return true
		case service.StateBlocked:
			return false
		default: //если статус не обработается из-за БД, пусть работает дальше
		}
		return true
	}

	// Fall back to invite UUID: token must map to a board, and pageID must
	// belong to that same board.
	board, err := svc.ValidateInvite(ctx, token)
	if err != nil {
		return false
	}
	for _, p := range board.Pages {
		if p.ID == pageID {
			return true
		}
	}
	return false
}

// parseTutorJWT mirrors middleware.Auth's validation. Returns the tutorID and
// true if token is a valid HS256 access token carrying an "id" claim.
func (m *WbHubManager) parseTutorJWT(tokenStr string) (string, bool) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		jwt.MapClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(m.jwtSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !token.Valid {
		return "", false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}
	tutorID, ok := claims["id"].(string)
	if !ok || tutorID == "" {
		return "", false
	}
	return tutorID, true
}

// seedMessage собирает сообщение сида. Пустая страница тоже получает сообщение —
// с пустым списком элементов: клиент открывает себе право сохранять именно по
// сиду, и молчание тут заперло бы персист новой доски навсегда.
func seedMessage(state json.RawMessage) ([]byte, error) {
	if len(state) == 0 {
		state = json.RawMessage(`{"elements":[],"files":{}}`)
	}
	return json.Marshal(WbMsg{Type: "snapshot", Payload: state})
}

// Run гоняет фоновую запись элементов. Вызывается из main.go; на отмену
// контекста дописывает буфер и выходит.
func (m *WbHubManager) Run(ctx context.Context) { m.buf.Run(ctx) }

// CleanupTombstones — сигнатура под runIntervalLoop в main.go.
func (m *WbHubManager) CleanupTombstones(ctx context.Context) (int64, error) {
	return m.svc.DeleteOldTombstones(ctx)
}

// ServeWS — handler GET /ws/board/:pageId?token=...
// The route is public (browsers cannot set headers on a WebSocket handshake),
// so authorization is performed here from the ?token= query param.
func (m *WbHubManager) ServeWS(svc service.WhiteboardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("pageId")
		token := c.Query("token")

		if !m.Authorize(c.Request.Context(), svc, pageID, token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Сид собирается из board_elements (+ карта files). Страница, ещё не
		// переведённую на поэлементную модель, переезжает здесь же.
		//
		// Ошибку чтения глотать нельзя: клиент сохраняет доску только после
		// сида, и молчание здесь — последний рубеж, удерживающий его от записи
		// пустой сцены (см. комментарий у отправки ниже).
		state, snapErr := svc.GetPageState(c.Request.Context(), pageID)
		if snapErr != nil {
			m.log.Error("read board state for seed", slog.String("error", snapErr.Error()), slog.String("page_id", pageID))
		}

		conn, err := m.upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &wbClient{
			conn:    conn,
			send:    make(chan []byte, 256),
			peerID:  uuid.New().String(),
			limiter: rate.NewLimiter(rate.Limit(wbByteRateLimit), wbByteRateBurst),
		}

		// Сидинг ДО регистрации: снапшот лежит первым в буфере send, поэтому
		// клиент гарантированно получит его раньше любого чужого update.
		//
		// Сообщение шлётся ВСЕГДА, когда чтение удалось, — в том числе для
		// пустой страницы. Клиент по нему открывает себе право сохранять: до
		// сида он постит пустую сцену смонтированного Excalidraw. Стереть доску
		// этим он больше не может (персист стал merge), но лишний холостой
		// раунд-трип не нужен и тут.
		if snapErr == nil {
			msg, err := seedMessage(state)
			if err != nil {
				// Битая запись в БД: не JSON. Тот же выбор, что и выше.
				m.log.Error("marshal board seed", slog.String("error", err.Error()), slog.String("page_id", pageID))
			} else {
				client.send <- msg
			}
		}

		// Register with the hub, retrying with a fresh hub if the one we got is
		// in the middle of shutting down (avoids the register-after-close race).
		for {
			hub := m.getOrCreate(pageID)
			client.hub = hub
			select {
			case hub.register <- client:
				goto registered
			case <-hub.done:
				// Hub died before our registration landed; loop for a new one.
			}
		}
	registered:

		go client.writePump()
		client.readPump()
	}
}
