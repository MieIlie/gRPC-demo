package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type AuthResponse struct {
	UserId       string `json:"user_id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

func main() {
	host := "gateway:8080"
	
	fmt.Printf("--- STARTING INTEGRATION TEST (target: %s) ---\n", host)

	// 1. REGISTER
	registerURL := fmt.Sprintf("http://%s/api/auth/register", host)
	regPayload := `{"username":"bob","password":"password123","display_name":"Bob Master","avatar_url":"http://avatar.url"}`
	
	fmt.Printf("[1] Registering user 'bob'...\n")
	resp, err := http.Post(registerURL, "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		fmt.Printf("Register request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		fmt.Printf("Register returned status: %d\n", resp.StatusCode)
		return
	}
	fmt.Printf("Register response status: %s\n", resp.Status)

	// 2. LOGIN
	loginURL := fmt.Sprintf("http://%s/api/auth/login", host)
	loginPayload := `{"username":"bob","password":"password123"}`
	
	fmt.Printf("[2] Logging in user 'bob'...\n")
	resp, err = http.Post(loginURL, "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		fmt.Printf("Login request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Login returned status: %d\n", resp.StatusCode)
		return
	}

	var authResult AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResult); err != nil {
		fmt.Printf("Failed to decode login response: %v\n", err)
		return
	}
	fmt.Printf("Login successful! User ID: %s, Username: %s\n", authResult.UserId, authResult.Username)
	token := authResult.Token

	// 3. GET PROTECTED
	protectedURL := fmt.Sprintf("http://%s/api/protected", host)
	req, _ := http.NewRequest("GET", protectedURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Printf("[3] Accessing protected API route...\n")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		fmt.Printf("Protected route request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Protected route response status: %s\n", resp.Status)
	var protectedResult map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&protectedResult)
	fmt.Printf("Protected payload returned: %+v\n", protectedResult)

	// 4. WEBSOCKET TEST WITH ENVELOPE
	wsURL := url.URL{Scheme: "ws", Host: host, Path: "/ws", RawQuery: "token=" + token}
	fmt.Printf("[4] Connecting to WebSocket: %s\n", wsURL.String())

	c, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		fmt.Printf("WebSocket connection failed: %v\n", err)
		return
	}
	defer c.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			fmt.Printf("WS Received: %s\n", string(message))
		}
	}()

	time.Sleep(500 * time.Millisecond)

	chatEventPayload := `{"roomId":"a3333333-3333-3333-3333-333333333333","content":"Hello from bob!"}`
	wsMsg := fmt.Sprintf(`{"event":"chat.send","data":%s}`, chatEventPayload)

	fmt.Printf("WS Sending: %s\n", wsMsg)
	err = c.WriteMessage(websocket.TextMessage, []byte(wsMsg))
	if err != nil {
		fmt.Printf("WS Write error: %v\n", err)
		return
	}

	time.Sleep(1 * time.Second)
	fmt.Printf("--- INTEGRATION TEST FINISHED SUCCESSFULLY ---\n")
}
