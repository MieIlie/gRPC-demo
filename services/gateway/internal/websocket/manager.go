package websocket

import (
	"shared/logger"
	"sync"
)

type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

func (m *Manager) Register(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Terminate existing connection if user re-connects
	if old, exists := m.clients[c.ID]; exists {
		logger.Info("Closing duplicate WebSocket connection for user: %s", c.ID)
		_ = old.Conn.Close()
	}

	m.clients[c.ID] = c
	logger.Info("User %s registered. Total online: %d", c.ID, len(m.clients))
}

func (m *Manager) Unregister(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[userID]; exists {
		delete(m.clients, userID)
		logger.Info("User %s unregistered. Total online: %d", userID, len(m.clients))
	}
}

func (m *Manager) SendMessage(userID string, message []byte) bool {
	m.mu.RLock()
	client, exists := m.clients[userID]
	m.mu.RUnlock()

	if exists {
		select {
		case client.Send <- message:
			return true
		default:
			logger.Error("Client send buffer full for user %s, dropping message", userID)
			return false
		}
	}
	return false
}

func (m *Manager) BroadcastMessage(message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for userID, client := range m.clients {
		select {
		case client.Send <- message:
		default:
			logger.Error("Client send buffer full for user %s during broadcast", userID)
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
