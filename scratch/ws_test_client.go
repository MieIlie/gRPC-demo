package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODEyMzI0MDAsInVzZXJfaWQiOiJkMWQ5MzMzMy0zMzMzLTMzMzMtMzMzMy0zMzMzMzMzMzMzMzMiLCJ1c2VybmFtZSI6InRlc3RfdXNlciJ9.iOOwUYmFI9Jqg1f2YALjAnl8STZk3cU5pRmpz8Hohvw"
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws", RawQuery: "token=" + token}
	fmt.Printf("Dialing server at: %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Printf("Dial error: %v\n", err)
		return
	}
	defer c.Close()

	done := make(chan struct{})

	// Start reading server messages in background
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Printf("Read error (connection closed): %v\n", err)
				return
			}
			fmt.Printf("Server Response: %s\n", message)
		}
	}()

	// Send message to server
	testMessage := "Hello Gateway Realtime!"
	fmt.Printf("Sending message to server: %s\n", testMessage)
	err = c.WriteMessage(websocket.TextMessage, []byte(testMessage))
	if err != nil {
		fmt.Printf("Write error: %v\n", err)
		return
	}

	// Sleep to allow read loop to catch echo responses
	time.Sleep(2 * time.Second)

	fmt.Println("Closing connection gracefully...")
	_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	select {
	case <-done:
	case <-time.After(1 * time.Second):
	}
}
