package main

import (
	"net"
	"shared/logger"
)

func main() {
	logger.Info("Auth Service is starting...")
	logger.Info("Listening on TCP port :50051")
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Error starting Auth TCP listener: %v", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Error accepting connection on Auth Service: %v", err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			_, _ = c.Write([]byte("Auth Service connection established (placeholder)\n"))
		}(conn)
	}
}
