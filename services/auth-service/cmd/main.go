package main

import (
	"net"
	"os"

	"auth-service/internal/db"
	"auth-service/internal/server"
	"gen/auth"
	"shared/logger"

	"google.golang.org/grpc"
)

func main() {
	logger.Info("Auth Service is starting...")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://app:app@postgres:5432/distributed_chat?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "supersecret"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":50051"
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
		logger.Error("Error starting Auth TCP listener: %v", err)
		os.Exit(1)
	}
	defer listener.Close()

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	authServer := server.NewServer(database, jwtSecret)
	auth.RegisterAuthServiceServer(grpcServer, authServer)

	logger.Info("Auth Service gRPC server is ready.")
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("Auth Service gRPC server crashed: %v", err)
		os.Exit(1)
	}
}

