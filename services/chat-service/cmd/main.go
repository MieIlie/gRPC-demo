package main

import (
	"net"
	"shared/logger"
)

func main() {
	logger.Info("Chat Service is starting...")
	logger.Info("Listening on TCP port :50052")
	listener, err := net.Listen("tcp", ":50052")
	if err != nil {
		logger.Error("Error starting Chat TCP listener: %v", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Error accepting connection on Chat Service: %v", err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			_, _ = c.Write([]byte("Chat Service connection established (placeholder)\n"))
		}(conn)
	}
}
