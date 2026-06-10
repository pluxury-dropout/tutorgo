package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"tutorgo/service"

	"github.com/bep/debounce"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WbMsg — message between client and hub
type WbMsg struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	X       float64         `json:"x,omitempty"`
	Y       float64         `json:"y,omitempty"`
	PeerID  string          `json:"peerId,omitempty"`
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
	snapshot     json.RawMessage
	saveDebounce func(f func())
	svc          service.WhiteboardService
	log          *slog.Logger
}

type wbBroadcast struct {
	sender *wbClient
	data   []byte
}

func newWbHub(pageID string, snapshot json.RawMessage, svc service.WhiteboardService, log *slog.Logger) *wbHub {
	h := &wbHub{
		pageID:     pageID,
		clients:    make(map[*wbClient]bool),
		broadcast:  make(chan wbBroadcast, 64),
		register:   make(chan *wbClient),
		unregister: make(chan *wbClient),
		snapshot:   snapshot,
		svc:        svc,
		log:        log,
	}
	h.saveDebounce = debounce.New(2 * time.Second)
	return h
}

func (h *wbHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// Send current snapshot to new client
			if h.snapshot != nil {
				msg, _ := json.Marshal(WbMsg{Type: "snapshot", Payload: h.snapshot})
				client.send <- msg
			}

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// Immediately save snapshot on disconnect
				if h.snapshot != nil {
					snap := h.snapshot
					go func() {
						if err := h.svc.SaveSnapshot(context.Background(), h.pageID, snap); err != nil {
							h.log.Error("save snapshot on disconnect", slog.String("error", err.Error()))
						}
					}()
				}
			}

		case msg := <-h.broadcast:
			var parsed WbMsg
			if err := json.Unmarshal(msg.data, &parsed); err != nil {
				continue
			}

			if parsed.Type == "update" {
				h.snapshot = parsed.Payload
				// Debounced save to DB
				h.saveDebounce(func() {
					snap := h.snapshot
					if err := h.svc.SaveSnapshot(context.Background(), h.pageID, snap); err != nil {
						h.log.Error("save snapshot", slog.String("error", err.Error()))
					}
				})
			}

			// Add sender's peerId to cursor messages
			if parsed.Type == "cursor" {
				parsed.PeerID = msg.sender.peerID
				msg.data, _ = json.Marshal(parsed)
			}

			// Broadcast to all except sender
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
type WbHubManager struct {
	mu   sync.Mutex
	hubs map[string]*wbHub
	svc  service.WhiteboardService
	log  *slog.Logger
}

func NewWbHubManager(svc service.WhiteboardService, log *slog.Logger) *WbHubManager {
	return &WbHubManager{
		hubs: make(map[string]*wbHub),
		svc:  svc,
		log:  log,
	}
}

func (m *WbHubManager) getOrCreate(pageID string, snapshot json.RawMessage) *wbHub {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hub, ok := m.hubs[pageID]; ok {
		return hub
	}
	hub := newWbHub(pageID, snapshot, m.svc, m.log)
	m.hubs[pageID] = hub
	go hub.run()
	return hub
}

// ServeWS — handler GET /ws/board/:pageId?token=...
func (m *WbHubManager) ServeWS(svc service.WhiteboardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("pageId")
		token := c.Query("token")

		// Authorization: JWT or invite UUID
		var authorized bool
		if c.GetString("tutorID") != "" {
			authorized = true
		} else if token != "" {
			// Validate invite token
			if _, err := svc.ValidateInvite(c.Request.Context(), token); err == nil {
				authorized = true
			}
		}
		if !authorized {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Load current snapshot from DB
		snapshot, _ := svc.GetPageSnapshot(c.Request.Context(), pageID)

		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		hub := m.getOrCreate(pageID, snapshot)
		client := &wbClient{
			hub:    hub,
			conn:   conn,
			send:   make(chan []byte, 256),
			peerID: uuid.New().String(),
		}
		hub.register <- client

		go client.writePump()
		client.readPump()
	}
}
