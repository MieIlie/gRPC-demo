package websocket

import (
	"encoding/json"
	"gateway/internal/trace"
	"gateway/internal/types"
	"shared/logger"
	"sync"
)

type Manager struct {
	mu           sync.RWMutex
	clients      map[string]map[*Client]bool
}

func NewManager() *Manager {
	return &Manager{
		clients:      make(map[string]map[*Client]bool),
	}
}

func (m *Manager) Register(c *Client) {
	m.mu.Lock()
	isFirstConn := false
	if _, exists := m.clients[c.ID]; !exists {
		m.clients[c.ID] = make(map[*Client]bool)
		isFirstConn = true
	}
	m.clients[c.ID][c] = true

	onlineUsers := make([]string, 0, len(m.clients))
	for userID := range m.clients {
		onlineUsers = append(onlineUsers, userID)
	}
	m.mu.Unlock()

	logger.Info("User %s registered (conn: %p). Total users: %d", c.ID, c.Conn, len(onlineUsers))

	trace.GetTracker().Record(&trace.Event{
		Source:   "Client (" + c.ID + ")",
		Target:   "Gateway",
		Protocol: "WebSocket",
		Type:     "Connect",
		Message:  "User connected and registered",
		Status:   "success",
	})

	// 1. Send the newly connected client the current online users list
	listData, _ := json.Marshal(onlineUsers)
	listEnvelope := types.WSMessage{
		Event: "user.online_list",
		Data:  listData,
	}
	listBytes, _ := json.Marshal(listEnvelope)
	select {
	case c.Send <- listBytes:
	default:
	}

	// 2. If it's the user's first connection, broadcast user.online to all other clients
	if isFirstConn {
		onlineData, _ := json.Marshal(map[string]string{"userId": c.ID})
		onlineEnvelope := types.WSMessage{
			Event: "user.online",
			Data:  onlineData,
		}
		onlineBytes, _ := json.Marshal(onlineEnvelope)

		m.mu.RLock()
		var clientsToNotify []*Client
		for uID, conns := range m.clients {
			if uID != c.ID {
				for client := range conns {
					clientsToNotify = append(clientsToNotify, client)
				}
			}
		}
		m.mu.RUnlock()

		for _, client := range clientsToNotify {
			select {
			case client.Send <- onlineBytes:
			default:
			}
		}
	}
}

func (m *Manager) Unregister(c *Client) {
	m.mu.Lock()
	isLastConn := false
	if conns, exists := m.clients[c.ID]; exists {
		delete(conns, c)
		if len(conns) == 0 {
			delete(m.clients, c.ID)
			isLastConn = true
		}
	}
	totalUsers := len(m.clients)
	m.mu.Unlock()

	logger.Info("User %s unregistered (conn: %p). Total users: %d", c.ID, c.Conn, totalUsers)

	trace.GetTracker().Record(&trace.Event{
		Source:   "Client (" + c.ID + ")",
		Target:   "Gateway",
		Protocol: "WebSocket",
		Type:     "Disconnect",
		Message:  "User disconnected and unregistered",
		Status:   "success",
	})

	// 3. If it's the user's last connection, broadcast user.offline to all other clients
	if isLastConn {
		offlineData, _ := json.Marshal(map[string]string{"userId": c.ID})
		offlineEnvelope := types.WSMessage{
			Event: "user.offline",
			Data:  offlineData,
		}
		offlineBytes, _ := json.Marshal(offlineEnvelope)

		m.mu.RLock()
		var clientsToNotify []*Client
		for _, conns := range m.clients {
			for client := range conns {
				clientsToNotify = append(clientsToNotify, client)
			}
		}
		m.mu.RUnlock()

		for _, client := range clientsToNotify {
			select {
			case client.Send <- offlineBytes:
			default:
			}
		}
	}
}

func (m *Manager) SendMessage(userID string, message []byte) bool {
	m.mu.RLock()
	conns, exists := m.clients[userID]
	if !exists || len(conns) == 0 {
		m.mu.RUnlock()
		return false
	}

	clients := make([]*Client, 0, len(conns))
	for client := range conns {
		clients = append(clients, client)
	}
	m.mu.RUnlock()

	success := false
	for _, client := range clients {
		select {
		case client.Send <- message:
			success = true
		default:
			logger.Error("Client send buffer full for user %s (conn: %p), dropping message", userID, client.Conn)
		}
	}
	return success
}

func (m *Manager) BroadcastMessage(message []byte) {
	m.mu.RLock()
	var clients []*Client
	for _, conns := range m.clients {
		for client := range conns {
			clients = append(clients, client)
		}
	}
	m.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.Send <- message:
		default:
			logger.Error("Client send buffer full for user %s (conn: %p) during broadcast", client.ID, client.Conn)
		}
	}
}

func (m *Manager) GetOnlineUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]string, 0, len(m.clients))
	for userID := range m.clients {
		users = append(users, userID)
	}
	return users
}

