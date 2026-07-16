package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gen/chat"
	"gateway/internal/config"
	"gateway/internal/grpc"
	"gateway/internal/middleware"
	"gateway/internal/trace"
	"gateway/internal/websocket"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type Router struct {
	config     *config.Config
	wsManager  *websocket.Manager
	wsRouter   *websocket.Router
	authClient *grpc.AuthClient
	chatClient *grpc.ChatClient
	mux        *http.ServeMux
}

func NewRouter(cfg *config.Config, wsMgr *websocket.Manager, authCli *grpc.AuthClient, chatCli *grpc.ChatClient) *Router {
	r := &Router{
		config:     cfg,
		wsManager:  wsMgr,
		wsRouter:   websocket.NewRouter(wsMgr, chatCli),
		authClient: authCli,
		chatClient: chatCli,
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
	r.mux.Handle("/uploads/", fs)

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

	// Traces API (Server-Sent Events)
	r.mux.HandleFunc("/api/traces", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		tracker := trace.GetTracker()
		ch := make(chan *trace.Event, 10)
		tracker.AddChannel(ch)
		defer tracker.RemoveChannel(ch)

		// Send history first
		history := tracker.GetEvents()
		for _, e := range history {
			data, err := json.Marshal(e)
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			}
		}
		flusher.Flush()

		ctx := req.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-ch:
				data, err := json.Marshal(e)
				if err == nil {
					_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
			}
		}
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

	// Protected Chat Endpoints
	protectedMux.HandleFunc("POST /api/rooms", func(w http.ResponseWriter, req *http.Request) {
		userID := middleware.GetUserID(req.Context())
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var payload struct {
			RoomType  int      `json:"room_type"`
			RoomName  string   `json:"room_name"`
			MemberIDs []string `json:"member_ids"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		rType := chat.RoomType(payload.RoomType)
		resp, err := r.chatClient.CreateRoom(req.Context(), rType, payload.RoomName, userID, payload.MemberIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	protectedMux.HandleFunc("GET /api/rooms", func(w http.ResponseWriter, req *http.Request) {
		userID := middleware.GetUserID(req.Context())
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		resp, err := r.chatClient.GetRooms(req.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	protectedMux.HandleFunc("GET /api/rooms/{id}/messages", func(w http.ResponseWriter, req *http.Request) {
		roomID := req.PathValue("id")
		if roomID == "" {
			http.Error(w, "Room ID is required", http.StatusBadRequest)
			return
		}

		limitStr := req.URL.Query().Get("limit")
		var limit int32 = 50
		if limitStr != "" {
			var l int
			_, _ = fmt.Sscanf(limitStr, "%d", &l)
			if l > 0 {
				limit = int32(l)
			}
		}

		var beforeTime *timestamppb.Timestamp
		beforeStr := req.URL.Query().Get("before")
		if beforeStr != "" {
			t, err := time.Parse(time.RFC3339, beforeStr)
			if err == nil {
				beforeTime = timestamppb.New(t)
			}
		}

		resp, err := r.chatClient.GetMessages(req.Context(), roomID, limit, beforeTime)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	protectedMux.HandleFunc("GET /api/rooms/{id}/members", func(w http.ResponseWriter, req *http.Request) {
		roomID := req.PathValue("id")
		if roomID == "" {
			http.Error(w, "Room ID is required", http.StatusBadRequest)
			return
		}

		resp, err := r.chatClient.GetRoomMembers(req.Context(), roomID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	protectedMux.HandleFunc("POST /api/upload", func(w http.ResponseWriter, req *http.Request) {
		// Limit upload size to 50MB
		err := req.ParseMultipartForm(50 << 20)
		if err != nil {
			http.Error(w, "File too large (max 50MB)", http.StatusBadRequest)
			return
		}

		file, header, err := req.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to get file from form data", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileURL, err := r.chatClient.UploadFile(req.Context(), header.Filename, file)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to upload file via gRPC: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"url": fileURL,
		})
	})

	protectedMux.HandleFunc("/ws", websocket.HandleConnection(r.wsManager, r.wsRouter, r.config.AllowedOrigins))

	// Apply JWT authentication middleware to protected paths
	r.mux.Handle("/api/protected", jwtMiddleware(protectedMux))
	r.mux.Handle("/api/rooms", jwtMiddleware(protectedMux))
	r.mux.Handle("/api/rooms/", jwtMiddleware(protectedMux))
	r.mux.Handle("/api/upload", jwtMiddleware(protectedMux))
	r.mux.Handle("/ws", jwtMiddleware(protectedMux))
}

