// Package ws provides WebSocket connection management for realtime bidirectional communication.
package ws

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:    func(r *http.Request) bool { return true }, // tighten in prod
}

// Message is the wire format for WS messages.
type Message struct {
	Type    string          `json:"type"`   // e.g. "chat", "task_update", "ping"
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Conn wraps a gorilla websocket connection with a send queue.
type Conn struct {
	id     string
	userID string
	conn   *websocket.Conn
	send   chan Message
	hub    *Hub
}

func (c *Conn) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		hub.broadcast <- broadcastMsg{from: c, msg: msg}
	}
}

func (c *Conn) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteJSON(msg); err != nil {
			return
		}
	}
}

// Hub manages all active WebSocket connections.
type Hub struct {
	mu         sync.RWMutex
	conns      map[string]*Conn // connID → conn
	userConns  map[string][]*Conn // userID → conns
	register   chan *Conn
	unregister chan *Conn
	broadcast  chan broadcastMsg
	logger     *zap.Logger
}

type broadcastMsg struct {
	from *Conn
	msg  Message
}

func NewHub(logger *zap.Logger) *Hub {
	h := &Hub{
		conns:      make(map[string]*Conn),
		userConns:  make(map[string][]*Conn),
		register:   make(chan *Conn, 32),
		unregister: make(chan *Conn, 32),
		broadcast:  make(chan broadcastMsg, 256),
		logger:     logger,
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.conns[c.id] = c
			h.userConns[c.userID] = append(h.userConns[c.userID], c)
			h.mu.Unlock()
			h.logger.Debug("ws client connected", zap.String("id", c.id), zap.String("user", c.userID))

		case c := <-h.unregister:
			h.mu.Lock()
			delete(h.conns, c.id)
			close(c.send)
			h.mu.Unlock()

		case bm := <-h.broadcast:
			_ = bm // route to appropriate handler
		}
	}
}

// SendToUser delivers a message to all connections for a given user.
func (h *Hub) SendToUser(userID string, msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.userConns[userID] {
		select {
		case c.send <- msg:
		default:
		}
	}
}

// Handler returns the Gin handler for the WebSocket upgrade endpoint.
func (h *Hub) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid, _ := userID.(string)

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			h.logger.Error("ws upgrade failed", zap.Error(err))
			return
		}

		wsc := &Conn{
			id:     uuid.New().String(),
			userID: uid,
			conn:   conn,
			send:   make(chan Message, 64),
			hub:    h,
		}

		h.register <- wsc
		go wsc.writePump()
		wsc.readPump(h)
	}
}
