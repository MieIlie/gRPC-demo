package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"gateway/internal/config"
	"gateway/internal/grpc"
	"gateway/internal/middleware"
	"gateway/internal/websocket"
)

type Router struct {
	config     *config.Config
	wsManager  *websocket.Manager
	wsRouter   *websocket.Router
	authClient *grpc.AuthClient
	mux        *http.ServeMux
}

func NewRouter(cfg *config.Config, wsMgr *websocket.Manager, authCli *grpc.AuthClient) *Router {
	r := &Router{
		config:     cfg,
		wsManager:  wsMgr,
		wsRouter:   websocket.NewRouter(wsMgr),
		authClient: authCli,
		mux:        http.NewServeMux(),
	}
	r.registerRoutes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) registerRoutes() {
	// Serve static files from the ./frontend directory
	fs := http.FileServer(http.Dir("./frontend"))
	r.mux.Handle("/assets/", fs)
	r.mux.Handle("/components/", fs)

	// Public endpoint
	r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			fs.ServeHTTP(w, req)
			return
		}

		// Check if request is from a browser expecting HTML
		if strings.Contains(req.Header.Get("Accept"), "text/html") {
			http.ServeFile(w, req, "./frontend/index.html")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"message": "Gateway Service API",
		})
	})

	// Public Auth endpoints
	r.mux.HandleFunc("/api/auth/register", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Username    string `json:"username"`
			Password    string `json:"password"`
			DisplayName string `json:"display_name"`
			AvatarURL   string `json:"avatar_url"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		resp, err := r.authClient.Register(req.Context(), payload.Username, payload.Password, payload.DisplayName, payload.AvatarURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	r.mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		resp, err := r.authClient.Login(req.Context(), payload.Username, payload.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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
	protectedMux.HandleFunc("/ws", websocket.HandleConnection(r.wsManager, r.wsRouter, r.config.AllowedOrigins))

	// Apply JWT authentication middleware to protected paths
	r.mux.Handle("/api/protected", jwtMiddleware(protectedMux))
	r.mux.Handle("/ws", jwtMiddleware(protectedMux))
}

