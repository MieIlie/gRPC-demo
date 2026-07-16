package websocket

import (
	"shared/logger"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

type Client struct {
	ID      string
	Conn    *websocket.Conn
	Manager *Manager
	Router  *Router
	Send    chan []byte
}

func NewClient(id string, conn *websocket.Conn, manager *Manager, router *Router) *Client {
	return &Client{
		ID:      id,
		Conn:    conn,
		Manager: manager,
		Router:  router,
		Send:    make(chan []byte, 256),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Manager.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				logger.Error("WebSocket client %s read error: %v", c.ID, err)
			}
			break
		}

		logger.Info("WebSocket client %s sent message: %s", c.ID, string(message))
		
		// Route message using the WebSocket Router
		c.Router.RouteMessage(c, message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The manager closed the channel
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send this message as its own WebSocket frame.
			// IMPORTANT: do NOT batch multiple messages into one frame (the old
			// NextWriter + flush-queued approach). Batching concatenates JSON
			// objects with '\n', which makes JSON.parse fail on the frontend.
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Drain any other queued messages, each as a separate frame.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.Conn.WriteMessage(websocket.TextMessage, <-c.Send); err != nil {
					return
				}
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

