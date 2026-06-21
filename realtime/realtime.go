package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"gobaas/db"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
		if allowedOrigins == "" {
			return true // Development mode — izinkan semua origin
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(allowed) == origin {
				return true
			}
		}
		return false
	},
}

type Client struct {
	conn        *websocket.Conn
	projectID   string
	subscribed  map[string]bool // Menyimpan nama tabel yang di-subscribe
	mu          sync.Mutex
	send        chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

var GlobalHub = &Hub{
	clients:    make(map[*Client]bool),
	register:   make(chan *Client),
	unregister: make(chan *Client),
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client terhubung ke Realtime. Project: %s", client.projectID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("Client terputus dari Realtime. Project: %s", client.projectID)
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast mengirimkan event perubahan data ke semua client yang berhak (berdasarkan project_id dan subscription tabel)
func (h *Hub) Broadcast(projectID, tableName, action string, record interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	payload := map[string]interface{}{
		"table":  tableName,
		"action": action, // INSERT, UPDATE, DELETE
		"record": record,
	}

	messageBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Gagal marshal payload realtime: %v", err)
		return
	}

	for client := range h.clients {
		if client.projectID == projectID {
			client.mu.Lock()
			// Hanya kirim jika client men-subscribe tabel tersebut
			if client.subscribed[tableName] {
				select {
				case client.send <- messageBytes:
				default:
					// Jika buffer penuh, putuskan client
					go func(c *Client) {
						h.unregister <- c
						c.conn.Close()
					}(client)
				}
			}
			client.mu.Unlock()
		}
	}
}

type SocketMessage struct {
	Event string `json:"event"` // "subscribe" atau "unsubscribe"
	Table string `json:"table"`
}

func (c *Client) readPump() {
	defer func() {
		GlobalHub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg SocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.send <- []byte(`{"error": "Format pesan tidak valid"}`)
			continue
		}

		c.mu.Lock()
		switch msg.Event {
		case "subscribe":
			c.subscribed[msg.Table] = true
			c.send <- []byte(`{"status": "subscribed", "table": "` + msg.Table + `"}`)
			log.Printf("Project %s mensubscribe tabel %s", c.projectID, msg.Table)
		case "unsubscribe":
			delete(c.subscribed, msg.Table)
			c.send <- []byte(`{"status": "unsubscribed", "table": "` + msg.Table + `"}`)
		}
		c.mu.Unlock()
	}
}

func (c *Client) writePump() {
	for message := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
}

// HandleRealtime mengelola request websocket
func HandleRealtime(w http.ResponseWriter, r *http.Request) {
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		http.Error(w, "Query parameter apikey dibutuhkan", http.StatusUnauthorized)
		return
	}

	var projectID string
	err := db.DB.QueryRow("SELECT id FROM projects WHERE api_key = $1", apiKey).Scan(&projectID)
	if err != nil {
		http.Error(w, "API Key tidak valid", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Gagal upgrade websocket: %v", err)
		return
	}

	client := &Client{
		conn:       conn,
		projectID:  projectID,
		subscribed: make(map[string]bool),
		send:       make(chan []byte, 256),
	}

	GlobalHub.register <- client

	go client.writePump()
	go client.readPump()
}
