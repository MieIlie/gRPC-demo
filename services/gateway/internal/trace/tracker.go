package trace

import (
	"sync"
	"time"
)

type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	Protocol   string    `json:"protocol"` // "gRPC", "WebSocket", "HTTP"
	Type       string    `json:"type"`     // "Request", "Response", "Receive", "Send", "Event"
	Message    string    `json:"message"`
	Status     string    `json:"status"`      // "success", "error", "pending"
	DurationMs int64     `json:"duration_ms"` // duration in milliseconds
}

type Tracker struct {
	mu       sync.RWMutex
	events   []*Event
	maxLogs  int
	channels []chan *Event
}

var (
	globalTracker *Tracker
	once          sync.Once
)

func GetTracker() *Tracker {
	once.Do(func() {
		globalTracker = &Tracker{
			events:   make([]*Event, 0),
			maxLogs:  100,
			channels: make([]chan *Event, 0),
		}
	})
	return globalTracker
}

func (t *Tracker) Record(e *Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e.Timestamp = time.Now()
	t.events = append(t.events, e)
	if len(t.events) > t.maxLogs {
		t.events = t.events[1:]
	}

	// Broadcast to active SSE streams
	for _, ch := range t.channels {
		select {
		case ch <- e:
		default:
			// avoid blocking if channel is full
		}
	}
}

func (t *Tracker) GetEvents() []*Event {
	t.mu.RLock()
	defer t.mu.RUnlock()

	copied := make([]*Event, len(t.events))
	copy(copied, t.events)
	return copied
}

func (t *Tracker) AddChannel(ch chan *Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channels = append(t.channels, ch)
}

func (t *Tracker) RemoveChannel(ch chan *Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, c := range t.channels {
		if c == ch {
			t.channels = append(t.channels[:i], t.channels[i+1:]...)
			break
		}
	}
}
