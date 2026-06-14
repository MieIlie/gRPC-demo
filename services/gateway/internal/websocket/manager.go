package websocket

import (
	"gateway/internal/trace"
	"shared/logger"
	"sync"
)

type Manager struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]bool
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]map[*Client]bool),
	}
}

func (m *Manager) Register(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[c.ID]; !exists {
		m.clients[c.ID] = make(map[*Client]bool)
	}
	m.clients[c.ID][c] = true
	logger.Info("User %s registered (conn: %p). Total users: %d", c.ID, c.Conn, len(m.clients))

	trace.GetTracker().Record(&trace.Event{
		Source:   "Client (" + c.ID + ")",
		Target:   "Gateway",
		Protocol: "WebSocket",
		Type:     "Connect",
		Message:  "User connected and registered",
		Status:   "success",
	})
}

func (m *Manager) Unregister(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conns, exists := m.clients[c.ID]; exists {
		delete(conns, c)
		if len(conns) == 0 {
			delete(m.clients, c.ID)
		}
		logger.Info("User %s unregistered (conn: %p). Total users: %d", c.ID, c.Conn, len(m.clients))

		trace.GetTracker().Record(&trace.Event{
			Source:   "Client (" + c.ID + ")",
			Target:   "Gateway",
			Protocol: "WebSocket",
			Type:     "Disconnect",
			Message:  "User disconnected and unregistered",
			Status:   "success",
		})
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

