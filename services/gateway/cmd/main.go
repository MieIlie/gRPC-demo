package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shared/logger"
	"gateway/internal/config"
	"gateway/internal/grpc"
	"gateway/internal/server"
	"gateway/internal/websocket"
)

func main() {
	cfg := config.Load()
	logger.Info("Starting Gateway Service on %s", cfg.Port)

	// Initialize gRPC Client to Auth Service
	authClient, err := grpc.NewAuthClient(cfg.AuthServiceAddr)
	if err != nil {
		logger.Error("Failed to initialize Auth Service client: %v", err)
		os.Exit(1)
	}
	defer authClient.Close()

	wsManager := websocket.NewManager()
	router := server.NewRouter(cfg, wsManager, authClient)

	srv := &http.Server{
		Addr:    cfg.Port,
		Handler: router,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Gateway Router initialized. Ready to accept requests.")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Gateway server crashed: %v", err)
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("Shutting down Gateway Service gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Gateway shutdown failed: %v", err)
	} else {
		logger.Info("Gateway Service stopped cleanly.")
	}
}


