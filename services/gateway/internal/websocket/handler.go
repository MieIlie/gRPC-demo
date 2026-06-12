package websocket

import (
	"net/http"
	"shared/logger"
	"gateway/internal/middleware"

	"github.com/gorilla/websocket"
)

func HandleConnection(manager *Manager, router *Router, allowedOrigins []string) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Allow non-browser requests
			}
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					return true
				}
			}
			return false
		},
	}

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

		client := NewClient(userID, conn, manager, router)
		manager.Register(client)

		// Spawn read and write loops in background goroutines
		go client.WritePump()
		go client.ReadPump()

		// Send initial welcome message
		client.Send <- []byte("Welcome to the Realtime Gateway Connection!")
	}
}

