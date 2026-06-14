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

type Room struct {
	Id       string `json:"id"`
	RoomType int    `json:"room_type"`
	RoomName string `json:"room_name"`
}

type CreateRoomResponse struct {
	Room Room `json:"room"`
}

type WSMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func main() {
	host := "localhost:8080"
	fmt.Printf("--- STARTING PHASE 4 INTEGRATION TEST (target: %s) ---\n", host)

	// 1. REGISTER USER A (Alice) AND USER B (Bob)
	aliceUsername := fmt.Sprintf("alice-%d", time.Now().UnixNano())
	bobUsername := fmt.Sprintf("bob-%d", time.Now().UnixNano())

	fmt.Printf("[1] Registering users '%s' and '%s'...\n", aliceUsername, bobUsername)
	aliceUser := registerUser(host, aliceUsername, "Alice Henderson")
	if aliceUser == nil {
		return
	}
	bobUser := registerUser(host, bobUsername, "Bob Vance")
	if bobUser == nil {
		return
	}

	// 2. CREATE GROUP ROOM (Alice + Bob)
	fmt.Printf("[2] Alice creating group room with Bob...\n")
	createRoomPayload := map[string]interface{}{
		"room_type":  1, // GROUP
		"room_name":  "Vance Refrigeration",
		"member_ids": []string{bobUser.UserId},
	}
	payloadBytes, _ := json.Marshal(createRoomPayload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/api/rooms", host), bytes.NewBuffer(payloadBytes))
	req.Header.Set("Authorization", "Bearer "+aliceUser.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to create group room: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Create group room returned status: %d\n", resp.StatusCode)
		return
	}

	var groupRoomResp CreateRoomResponse
	_ = json.NewDecoder(resp.Body).Decode(&groupRoomResp)
	fmt.Printf("Group room created! ID: %s, Name: %s\n", groupRoomResp.Room.Id, groupRoomResp.Room.RoomName)
	_ = groupRoomResp.Room.Id

	// 3. CREATE DIRECT ROOM (Alice + Bob) -> DM Prevention Test
	fmt.Printf("[3] Alice creating direct room 1 with Bob...\n")
	dmPayload := map[string]interface{}{
		"room_type":  0, // DIRECT
		"member_ids": []string{bobUser.UserId},
	}
	dmBytes, _ := json.Marshal(dmPayload)
	req, _ = http.NewRequest("POST", fmt.Sprintf("http://%s/api/rooms", host), bytes.NewBuffer(dmBytes))
	req.Header.Set("Authorization", "Bearer "+aliceUser.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to create direct room 1: %v\n", err)
		return
	}
	defer resp.Body.Close()
	var dmResp1 CreateRoomResponse
	_ = json.NewDecoder(resp.Body).Decode(&dmResp1)
	dmID1 := dmResp1.Room.Id
	fmt.Printf("Direct room 1 created! ID: %s\n", dmID1)

	fmt.Printf("[4] Alice creating direct room 2 with Bob (DM duplication check)...\n")
	req, _ = http.NewRequest("POST", fmt.Sprintf("http://%s/api/rooms", host), bytes.NewBuffer(dmBytes))
	req.Header.Set("Authorization", "Bearer "+aliceUser.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to create direct room 2: %v\n", err)
		return
	}
	defer resp.Body.Close()
	var dmResp2 CreateRoomResponse
	_ = json.NewDecoder(resp.Body).Decode(&dmResp2)
	dmID2 := dmResp2.Room.Id
	fmt.Printf("Direct room 2 returned! ID: %s\n", dmID2)

	if dmID1 != dmID2 {
		fmt.Printf("CRITICAL ERROR: Duplicate direct room created (ID1: %s, ID2: %s)\n", dmID1, dmID2)
		return
	}
	fmt.Printf("SUCCESS: Duplicate direct room prevented. Reused ID: %s\n", dmID1)

	// 4. WEBSOCKET MESSAGING TEST (Alice <--> Bob)
	fmt.Printf("[5] Connecting Bob and Alice to WebSockets...\n")
	bobWS := connectWS(host, bobUser.Token)
	if bobWS == nil {
		return
	}
	defer bobWS.Close()

	aliceWS := connectWS(host, aliceUser.Token)
	if aliceWS == nil {
		return
	}
	defer aliceWS.Close()

	// Listen for Bob receiving message
	bobReceived := make(chan string, 1)
	go func() {
		for {
			_, message, err := bobWS.ReadMessage()
			if err != nil {
				return
			}
			var wsMsg WSMessage
			if err := json.Unmarshal(message, &wsMsg); err == nil && wsMsg.Event == "chat.receive" {
				var data map[string]interface{}
				_ = json.Unmarshal(wsMsg.Data, &data)
				content, _ := data["content"].(string)
				bobReceived <- content
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)

	// Alice sends message to Bob in the Direct Room
	fmt.Printf("[6] Alice sending message to Bob over WebSocket in direct room...\n")
	chatEvent := map[string]interface{}{
		"roomId":  dmID1,
		"content": "Hi Bob, Alice here!",
	}
	chatBytes, _ := json.Marshal(chatEvent)
	aliceMsg := fmt.Sprintf(`{"event":"chat.send","data":%s}`, string(chatBytes))
	_ = aliceWS.WriteMessage(websocket.TextMessage, []byte(aliceMsg))

	select {
	case msg := <-bobReceived:
		fmt.Printf("SUCCESS: Bob received WebSocket message from Alice: '%s'\n", msg)
	case <-time.After(3 * time.Second):
		fmt.Printf("CRITICAL ERROR: Bob timed out waiting for Alice's message\n")
		return
	}

	// 5. SERVER-SIDE MEMBERSHIP AUTHORIZATION TEST (Bob attempts to send in private room)
	// Alice creates a private room with herself only
	fmt.Printf("[7] Alice creating private room with herself only...\n")
	privatePayload := map[string]interface{}{
		"room_type":  1, // GROUP
		"room_name":  "Alice Secret Vault",
		"member_ids": []string{},
	}
	privBytes, _ := json.Marshal(privatePayload)
	req, _ = http.NewRequest("POST", fmt.Sprintf("http://%s/api/rooms", host), bytes.NewBuffer(privBytes))
	req.Header.Set("Authorization", "Bearer "+aliceUser.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to create private room: %v\n", err)
		return
	}
	defer resp.Body.Close()
	var privResp CreateRoomResponse
	_ = json.NewDecoder(resp.Body).Decode(&privResp)
	privRoomID := privResp.Room.Id
	fmt.Printf("Private room created! ID: %s\n", privRoomID)

	// Bob listens for system.error
	bobErrorChan := make(chan string, 1)
	go func() {
		for {
			_, message, err := bobWS.ReadMessage()
			if err != nil {
				return
			}
			var wsMsg WSMessage
			if err := json.Unmarshal(message, &wsMsg); err == nil && wsMsg.Event == "system.error" {
				var data map[string]interface{}
				_ = json.Unmarshal(wsMsg.Data, &data)
				errMsg, _ := data["message"].(string)
				bobErrorChan <- errMsg
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)

	// Bob attempts to send message to Alice's private vault
	fmt.Printf("[8] Bob attempting unauthorized message injection in Alice's private room...\n")
	unauthEvent := map[string]interface{}{
		"roomId":  privRoomID,
		"content": "Aha! I am trying to hack this room!",
	}
	unauthBytes, _ := json.Marshal(unauthEvent)
	bobMsg := fmt.Sprintf(`{"event":"chat.send","data":%s}`, string(unauthBytes))
	_ = bobWS.WriteMessage(websocket.TextMessage, []byte(bobMsg))

	select {
	case errMsg := <-bobErrorChan:
		fmt.Printf("SUCCESS: Bob's message was blocked. Received system.error: '%s'\n", errMsg)
	case <-time.After(3 * time.Second):
		fmt.Printf("CRITICAL ERROR: Bob's message was NOT blocked or no system.error received\n")
		return
	}

	// 6. QUERY MESSAGE HISTORY CURSOR TEST
	fmt.Printf("[9] Querying direct room message history...\n")
	req, _ = http.NewRequest("GET", fmt.Sprintf("http://%s/api/rooms/%s/messages?limit=10", host, dmID1), nil)
	req.Header.Set("Authorization", "Bearer "+aliceUser.Token)
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to query message history: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var historyResp struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&historyResp)
	fmt.Printf("Fetched messages history count: %d\n", len(historyResp.Messages))
	if len(historyResp.Messages) > 0 {
		fmt.Printf("Latest message content: '%s'\n", historyResp.Messages[0].Content)
	}

	fmt.Printf("--- PHASE 4 INTEGRATION TEST FINISHED SUCCESSFULLY ---\n")
}

func registerUser(host, username, displayName string) *AuthResponse {
	registerURL := fmt.Sprintf("http://%s/api/auth/register", host)
	regPayload := fmt.Sprintf(`{"username":"%s","password":"password123","display_name":"%s","avatar_url":"http://avatar.url"}`, username, displayName)
	
	resp, err := http.Post(registerURL, "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		fmt.Printf("Register request failed: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Register returned status: %d\n", resp.StatusCode)
		return nil
	}

	var authResult AuthResponse
	_ = json.NewDecoder(resp.Body).Decode(&authResult)
	return &authResult
}

func connectWS(host, token string) *websocket.Conn {
	wsURL := url.URL{Scheme: "ws", Host: host, Path: "/ws", RawQuery: "token=" + token}
	c, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		fmt.Printf("WebSocket connection failed: %v\n", err)
		return nil
	}
	
	// Read welcome message
	_, _, _ = c.ReadMessage()
	return c
}
