package main

import (
	"net"
	"shared/logger"
)

func main() {
	logger.Info("Call Service is starting...")
	logger.Info("Listening on TCP port :50053")
	listener, err := net.Listen("tcp", ":50053")
	if err != nil {
		logger.Error("Error starting Call TCP listener: %v", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Error accepting connection on Call Service: %v", err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			_, _ = c.Write([]byte("Call Service connection established (placeholder)\n"))
		}(conn)
	}
}
