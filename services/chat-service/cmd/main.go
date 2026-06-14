package main

import (
	"gen/chat"
	"net"
	"os"
	"chat-service/internal/db"
	"chat-service/internal/server"
	"shared/logger"

	"google.golang.org/grpc"
)

func main() {
	logger.Info("Chat Service is starting...")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://app:app@postgres:5432/distributed_chat?sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":50052"
	} else if port[0] != ':' {
		port = ":" + port
	}

	// Connect to database
	database, err := db.Connect(dbURL)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer database.Close()

	// Listen on TCP port
	logger.Info("Listening on TCP port %s", port)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		logger.Error("Error starting Chat TCP listener: %v", err)
		os.Exit(1)
	}
	defer listener.Close()

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	chatServer := server.NewChatServer(database)
	chat.RegisterChatServiceServer(grpcServer, chatServer)

	logger.Info("Chat Service gRPC server is ready.")
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("Chat Service gRPC server crashed: %v", err)
		os.Exit(1)
	}
}
