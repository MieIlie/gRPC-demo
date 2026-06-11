package websocket

import (
	"net/http"
	"shared/logger"
	"gateway/internal/middleware"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for dev environment
		return true
	},
}

func HandleConnection(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == "" {
			http.Error(w, "Unauthorized: user ID missing from context", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Failed to upgrade connection: %v", err)
			return
		}

		client := NewClient(userID, conn, manager)
		manager.Register(client)

		// Spawn read and write loops in background goroutines
		go client.WritePump()
		go client.ReadPump()

		// Send initial welcome message
		client.Send <- []byte("Welcome to the Realtime Gateway Connection!")
	}
}
