package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gen/chat"
	"gateway/internal/grpc"
	"gateway/internal/trace"
	"gateway/internal/types"
	"shared/logger"
)

type Router struct {
	manager    *Manager
	chatClient *grpc.ChatClient
}

func NewRouter(manager *Manager, chatCli *grpc.ChatClient) *Router {
	return &Router{
		manager:    manager,
		chatClient: chatCli,
	}
}

func (r *Router) RouteMessage(client *Client, rawMsg []byte) {
	var msg types.WSMessage
	if err := json.Unmarshal(rawMsg, &msg); err != nil {
		logger.Error("Failed to parse websocket message from client %s: %v", client.ID, err)
		return
	}

	logger.Info("Routing websocket event: %s from user: %s", msg.Event, client.ID)

	switch msg.Event {
	case "chat.send":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal chat.send data: %v", err)
			return
		}

		roomId, _ := data["roomId"].(string)
		content, _ := data["content"].(string)

		if roomId == "" || content == "" {
			r.sendError(client, "Room ID and content are required")
			return
		}

		// 1. Trace incoming message
		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + client.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Receive",
			Message:  fmt.Sprintf("chat.send { roomId: %s, content: %s }", roomId, content),
			Status:   "success",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 2. Call Chat Service via gRPC to validate membership and persist message
		chatResp, err := r.chatClient.SendMessage(ctx, roomId, client.ID, content, chat.MessageType_TEXT)
		if err != nil {
			logger.Error("Failed to send message via gRPC: %v", err)
			r.sendError(client, err.Error())
			return
		}

		persistedMsg := chatResp.GetMessage()
		createdAtStr := persistedMsg.GetCreatedAt().AsTime().Format(time.RFC3339)

		// 3. Query room members list from Chat Service via gRPC (Gateway does not own membership state)
		membersResp, err := r.chatClient.GetRoomMembers(ctx, roomId)
		if err != nil {
			logger.Error("Failed to fetch room members: %v", err)
			r.sendError(client, "Failed to broadcast message: unable to load room membership")
			return
		}

		// 4. Formulate standardized WS event envelope for broadcast
		respData := map[string]interface{}{
			"messageId": persistedMsg.GetId(),
			"roomId":    persistedMsg.GetRoomId(),
			"senderId":  persistedMsg.GetSenderId(),
			"content":   persistedMsg.GetContent(),
			"createdAt": createdAtStr,
		}
		respBytes, _ := json.Marshal(respData)
		envelope := types.WSMessage{
			Event: "chat.receive",
			Data:  respBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)

		// 5. Broadcast to online members of the room in non-blocking loop
		for _, member := range membersResp.GetMembers() {
			r.manager.SendMessage(member.GetUserId(), envelopeBytes)
		}

		// 6. Trace broadcast success
		trace.GetTracker().Record(&trace.Event{
			Source:   "Gateway",
			Target:   "Room Clients",
			Protocol: "WebSocket",
			Type:     "Broadcast",
			Message:  fmt.Sprintf("chat.receive { roomId: %s, senderId: %s, content: %s }", roomId, client.ID, content),
			Status:   "success",
		})

	default:
		logger.Error("Unhandled websocket event: %s", msg.Event)
		r.sendError(client, fmt.Sprintf("Unhandled event type: %s", msg.Event))
	}
}

func (r *Router) sendError(client *Client, errMsg string) {
	errData := map[string]string{"message": errMsg}
	errBytes, _ := json.Marshal(errData)
	envelope := types.WSMessage{
		Event: "system.error",
		Data:  errBytes,
	}
	envelopeBytes, _ := json.Marshal(envelope)
	select {
	case client.Send <- envelopeBytes:
	default:
		logger.Error("Client send buffer full for user %s, dropping system.error", client.ID)
	}

	trace.GetTracker().Record(&trace.Event{
		Source:   "Gateway",
		Target:   "Client (" + client.ID + ")",
		Protocol: "WebSocket",
		Type:     "Send",
		Message:  fmt.Sprintf("system.error: %s", errMsg),
		Status:   "error",
	})
}
