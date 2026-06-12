package websocket

import (
	"encoding/json"
	"gateway/internal/types"
	"shared/logger"
)

type Router struct {
	manager *Manager
}

func NewRouter(manager *Manager) *Router {
	return &Router{manager: manager}
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

		// Phase 2 placeholder: simulate delivery to all users
		respData := map[string]interface{}{
			"messageId": "msg-placeholder-id",
			"roomId":    data["roomId"],
			"senderId":  client.ID,
			"content":   data["content"],
			"createdAt": "now",
		}
		respBytes, _ := json.Marshal(respData)
		envelope := types.WSMessage{
			Event: "chat.receive",
			Data:  respBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)
		r.manager.BroadcastMessage(envelopeBytes)

	default:
		logger.Error("Unhandled websocket event: %s", msg.Event)
	}
}
