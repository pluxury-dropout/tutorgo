package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tutorgo/service"

	"github.com/bep/debounce"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
}

// wbHub — coordinator for one board page
type wbHub struct {
	pageID       string
	clients      map[*wbClient]bool
	broadcast    chan wbBroadcast
	register     chan *wbClient
	unregister   chan *wbClient
	done         chan struct{}
	snapshot     json.RawMessage
	saveDebounce func(f func())
	svc          service.WhiteboardService
	mgr          *WbHubManager
	log          *slog.Logger
}

type wbBroadcast struct {
	sender *wbClient
	data   []byte
}

func newWbHub(pageID string, snapshot json.RawMessage, mgr *WbHubManager) *wbHub {
	h := &wbHub{
		pageID:     pageID,
		clients:    make(map[*wbClient]bool),
		broadcast:  make(chan wbBroadcast, 64),
		register:   make(chan *wbClient),
		unregister: make(chan *wbClient),
		done:       make(chan struct{}),
		snapshot:   snapshot,
		svc:        mgr.svc,
		mgr:        mgr,
		log:        mgr.log,
	}
	h.saveDebounce = debounce.New(2 * time.Second)
	return h
}

func (h *wbHub) saveSnapshot(snap json.RawMessage) {
	if snap == nil {
		return
	}
	if err := h.svc.SaveSnapshot(context.Background(), h.pageID, snap); err != nil {
		h.log.Error("save snapshot", slog.String("error", err.Error()))
	}
}

func (h *wbHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// Send the current full document snapshot to the new client.
			if h.snapshot != nil {
				msg, _ := json.Marshal(WbMsg{Type: "snapshot", Payload: h.snapshot})
				client.send <- msg
			}

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
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
					// Capture snapshot value to avoid a data race with run().
					snap := h.snapshot
					h.saveSnapshot(snap)
					return
				}
				h.mgr.mu.Unlock()
			}

		case msg := <-h.broadcast:
			var parsed WbMsg
			if err := json.Unmarshal(msg.data, &parsed); err != nil {
				continue
			}

			switch parsed.Type {
			case "snapshot":
				// Full document snapshot: store and debounce-save, do NOT relay.
				h.snapshot = parsed.Payload
				snap := h.snapshot
				h.saveDebounce(func() { h.saveSnapshot(snap) })
				continue

			case "cursor", "viewport":
				// Attach sender's peerId, then relay. Follow-mode matches the
				// viewport sender against the follower's userToFollow.socketId,
				// which equals this same peerId (learned via cursor messages).
				parsed.PeerID = msg.sender.peerID
				msg.data, _ = json.Marshal(parsed)

			case "update":
				// Incremental diff: relay verbatim, never store or persist.

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
		}
	}
}

func (c *wbClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
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
	}
	m.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     m.checkOrigin,
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

func (m *WbHubManager) getOrCreate(pageID string, snapshot json.RawMessage) *wbHub {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hub, ok := m.hubs[pageID]; ok {
		return hub
	}
	hub := newWbHub(pageID, snapshot, m)
	m.hubs[pageID] = hub
	go hub.run()
	return hub
}

// PushToPage вбрасывает серверное сообщение в хаб страницы. Если хаба нет —
// никто не подключён, и слать некому: молча выходим (клиент увидит контент из
// снапшота/S3 при следующем подключении). sender=nil ⇒ run() раздаст всем.
// ВАЖНО: только для типов, которые run() ретранслирует вербатим (default-ветка);
// cursor/viewport разыменовывают sender и с nil упадут.
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

// authorizeWS validates the ?token= query param. It first tries to parse it as a
// tutor access JWT (same scheme as middleware.Auth); on success it requires that
// pageID's board belongs to the tutor. Otherwise it treats the token as an invite
// UUID and requires that pageID belongs to the invite's board. Returns true iff
// the client may access the page.
func (m *WbHubManager) authorizeWS(ctx context.Context, svc service.WhiteboardService, pageID, token string) bool {
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

// ServeWS — handler GET /ws/board/:pageId?token=...
// The route is public (browsers cannot set headers on a WebSocket handshake),
// so authorization is performed here from the ?token= query param.
func (m *WbHubManager) ServeWS(svc service.WhiteboardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("pageId")
		token := c.Query("token")

		if !m.authorizeWS(c.Request.Context(), svc, pageID, token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Load current snapshot from DB (full document).
		snapshot, _ := svc.GetPageSnapshot(c.Request.Context(), pageID)

		conn, err := m.upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &wbClient{
			conn:   conn,
			send:   make(chan []byte, 256),
			peerID: uuid.New().String(),
		}

		// Register with the hub, retrying with a fresh hub if the one we got is
		// in the middle of shutting down (avoids the register-after-close race).
		for {
			hub := m.getOrCreate(pageID, snapshot)
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
