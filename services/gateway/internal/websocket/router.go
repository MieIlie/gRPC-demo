package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gen/call"
	"gen/chat"
	"gateway/internal/grpc"
	"gateway/internal/trace"
	"gateway/internal/types"
	"shared/logger"
)

type Router struct {
	manager    *Manager
	chatClient *grpc.ChatClient
	callClient *grpc.CallClient
}

func NewRouter(manager *Manager, chatCli *grpc.ChatClient, callCli *grpc.CallClient) *Router {
	return &Router{
		manager:    manager,
		chatClient: chatCli,
		callClient: callCli,
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

	case "chat.typing":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal chat.typing data: %v", err)
			return
		}

		roomId, _ := data["roomId"].(string)
		if roomId == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		membersResp, err := r.chatClient.GetRoomMembers(ctx, roomId)
		if err != nil {
			logger.Error("Failed to fetch room members for typing indicator: %v", err)
			return
		}

		respData := map[string]interface{}{
			"roomId": roomId,
			"userId": client.ID,
		}
		respBytes, _ := json.Marshal(respData)
		envelope := types.WSMessage{
			Event: "chat.typing",
			Data:  respBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)

		for _, member := range membersResp.GetMembers() {
			if member.GetUserId() != client.ID {
				r.manager.SendMessage(member.GetUserId(), envelopeBytes)
			}
		}

	case "call.start":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal call.start data: %v", err)
			return
		}

		roomId, _ := data["roomId"].(string)
		targetUserId, _ := data["targetUserId"].(string)
		callTypeStr, _ := data["callType"].(string)

		if targetUserId == "" {
			r.sendError(client, "Target User ID is required")
			return
		}

		// Verify target user is online
		online := r.manager.GetOnlineUsers()
		targetOnline := false
		for _, uID := range online {
			if uID == targetUserId {
				targetOnline = true
				break
			}
		}
		if !targetOnline {
			r.sendError(client, "User is offline")
			return
		}

		cType := call.CallType_VOICE
		if callTypeStr == "video" {
			cType = call.CallType_VIDEO
		}

		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + client.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Receive",
			Message:  fmt.Sprintf("call.start { roomId: %s, targetUserId: %s, callType: %s }", roomId, targetUserId, callTypeStr),
			Status:   "success",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := r.callClient.StartCall(ctx, roomId, client.ID, targetUserId, cType)
		if err != nil {
			logger.Error("Failed to start call: %v", err)
			r.sendError(client, "Failed to start call")
			return
		}

		callId := resp.GetSession().GetId()

		// Send call.incoming to target receiver
		incomingData := map[string]interface{} {
			"callId":   callId,
			"callerId": client.ID,
			"callType": callTypeStr,
		}
		incBytes, _ := json.Marshal(incomingData)
		envelope := types.WSMessage{
			Event: "call.incoming",
			Data:  incBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)
		r.manager.SendMessage(targetUserId, envelopeBytes)

		trace.GetTracker().Record(&trace.Event{
			Source:   "Gateway",
			Target:   "Client (" + targetUserId + ")",
			Protocol: "WebSocket",
			Type:     "Send",
			Message:  fmt.Sprintf("call.incoming { callId: %s, callerId: %s }", callId, client.ID),
			Status:   "success",
		})

	case "call.accept":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal call.accept data: %v", err)
			return
		}

		callId, _ := data["callId"].(string)
		if callId == "" {
			return
		}

		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + client.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Receive",
			Message:  fmt.Sprintf("call.accept { callId: %s }", callId),
			Status:   "success",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := r.callClient.AcceptCall(ctx, callId)
		if err != nil {
			logger.Error("Failed to accept call: %v", err)
			r.sendError(client, "Failed to accept call")
			return
		}

		callerId := resp.GetSession().GetCallerId()

		// Send call.accepted to caller
		acceptedData := map[string]interface{}{
			"callId": callId,
		}
		accBytes, _ := json.Marshal(acceptedData)
		envelope := types.WSMessage{
			Event: "call.accepted",
			Data:  accBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)
		r.manager.SendMessage(callerId, envelopeBytes)

		trace.GetTracker().Record(&trace.Event{
			Source:   "Gateway",
			Target:   "Client (" + callerId + ")",
			Protocol: "WebSocket",
			Type:     "Send",
			Message:  fmt.Sprintf("call.accepted { callId: %s }", callId),
			Status:   "success",
		})

	case "call.reject":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal call.reject data: %v", err)
			return
		}

		callId, _ := data["callId"].(string)
		if callId == "" {
			return
		}

		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + client.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Receive",
			Message:  fmt.Sprintf("call.reject { callId: %s }", callId),
			Status:   "success",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := r.callClient.RejectCall(ctx, callId)
		if err != nil {
			logger.Error("Failed to reject call: %v", err)
			r.sendError(client, "Failed to reject call")
			return
		}

		callerId := resp.GetSession().GetCallerId()

		// Send call.rejected to caller
		rejectedData := map[string]interface{}{
			"callId": callId,
		}
		rejBytes, _ := json.Marshal(rejectedData)
		envelope := types.WSMessage{
			Event: "call.rejected",
			Data:  rejBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)
		r.manager.SendMessage(callerId, envelopeBytes)

		trace.GetTracker().Record(&trace.Event{
			Source:   "Gateway",
			Target:   "Client (" + callerId + ")",
			Protocol: "WebSocket",
			Type:     "Send",
			Message:  fmt.Sprintf("call.rejected { callId: %s }", callId),
			Status:   "success",
		})

	case "call.end":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal call.end data: %v", err)
			return
		}

		callId, _ := data["callId"].(string)
		if callId == "" {
			return
		}

		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + client.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Receive",
			Message:  fmt.Sprintf("call.end { callId: %s }", callId),
			Status:   "success",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := r.callClient.EndCall(ctx, callId)
		if err != nil {
			logger.Error("Failed to end call: %v", err)
			r.sendError(client, "Failed to end call")
			return
		}

		callerId := resp.GetSession().GetCallerId()
		receiverId := resp.GetSession().GetReceiverId()

		// Send call.ended to both caller and receiver
		endedData := map[string]interface{}{
			"callId": callId,
		}
		endBytes, _ := json.Marshal(endedData)
		envelope := types.WSMessage{
			Event: "call.ended",
			Data:  endBytes,
		}
		envelopeBytes, _ := json.Marshal(envelope)

		r.manager.SendMessage(callerId, envelopeBytes)
		r.manager.SendMessage(receiverId, envelopeBytes)

		trace.GetTracker().Record(&trace.Event{
			Source:   "Gateway",
			Target:   "Call Clients",
			Protocol: "WebSocket",
			Type:     "Broadcast",
			Message:  fmt.Sprintf("call.ended { callId: %s }", callId),
			Status:   "success",
		})

	case "webrtc.offer", "webrtc.answer", "webrtc.ice-candidate":
		var data map[string]interface{}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			logger.Error("Failed to unmarshal %s data: %v", msg.Event, err)
			return
		}

		callId, _ := data["callId"].(string)
		if callId == "" {
			return
		}

		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + client.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Receive",
			Message:  fmt.Sprintf("%s { callId: %s }", msg.Event, callId),
			Status:   "success",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := r.callClient.GetCallSession(ctx, callId)
		if err != nil {
			logger.Error("Failed to fetch call session for signaling: %v", err)
			return
		}

		sess := resp.GetSession()
		var targetId string
		if sess.GetCallerId() == client.ID {
			targetId = sess.GetReceiverId()
		} else {
			targetId = sess.GetCallerId()
		}

		// Forward the message to the peer
		envelopeBytes, _ := json.Marshal(msg)
		r.manager.SendMessage(targetId, envelopeBytes)

		trace.GetTracker().Record(&trace.Event{
			Source:   "Gateway",
			Target:   "Client (" + targetId + ")",
			Protocol: "WebSocket",
			Type:     "Send",
			Message:  fmt.Sprintf("%s forwarded", msg.Event),
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
