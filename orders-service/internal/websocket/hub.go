package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true 
	},
}

type Hub struct {
	clients    map[int64]map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type Message struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan Message
	orderID  int64
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[int64]map[*Client]bool),
		broadcast: make(chan Message),
		register:  make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.orderID] == nil {
				h.clients[client.orderID] = make(map[*Client]bool)
			}
			h.clients[client.orderID][client] = true
			h.mu.Unlock()
			log.Printf("Client registered for order %d", client.orderID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.orderID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.orderID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("Client unregistered for order %d", client.orderID)

		case message := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.clients[message.OrderID]; ok {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastOrderUpdate(orderID int64, status string) {
	h.broadcast <- Message{
		OrderID: orderID,
		Status:  status,
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, orderID int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:     h,
		conn:    conn,
		send:    make(chan Message, 256),
		orderID: orderID,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Error writing message: %v", err)
				return
			}
		}
	}
}

