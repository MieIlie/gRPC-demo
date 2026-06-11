package server

import (
	"encoding/json"
	"net/http"
	"gateway/internal/config"
	"gateway/internal/middleware"
	"gateway/internal/websocket"
)

type Router struct {
	config    *config.Config
	wsManager *websocket.Manager
	mux       *http.ServeMux
}

func NewRouter(cfg *config.Config, wsMgr *websocket.Manager) *Router {
	r := &Router{
		config:    cfg,
		wsManager: wsMgr,
		mux:       http.NewServeMux(),
	}
	r.registerRoutes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) registerRoutes() {
	// Public endpoint
	r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"message": "Gateway Service API",
		})
	})

	// JWT auth middleware
	jwtMiddleware := middleware.JWTAuth(r.config.JWTSecret)

	// Create sub-router for protected routes
	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/api/protected", func(w http.ResponseWriter, req *http.Request) {
		userID := middleware.GetUserID(req.Context())
		username := middleware.GetUsername(req.Context())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "Access granted!",
			"user_id":  userID,
			"username": username,
		})
	})
	protectedMux.HandleFunc("/ws", websocket.HandleConnection(r.wsManager))

	// Apply JWT authentication middleware to protected paths
	r.mux.Handle("/api/protected", jwtMiddleware(protectedMux))
	r.mux.Handle("/ws", jwtMiddleware(protectedMux))
}
