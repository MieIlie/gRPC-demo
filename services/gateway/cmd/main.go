package main

import (
	"net/http"
	"shared/logger"
	"gateway/internal/config"
	"gateway/internal/server"
	"gateway/internal/websocket"
)

func main() {
	cfg := config.Load()
	logger.Info("Starting Gateway Service on %s", cfg.Port)

	wsManager := websocket.NewManager()
	router := server.NewRouter(cfg, wsManager)

	logger.Info("Gateway Router initialized. Ready to accept requests.")
	if err := http.ListenAndServe(cfg.Port, router); err != nil {
		logger.Error("Gateway server crashed: %v", err)
	}
}
