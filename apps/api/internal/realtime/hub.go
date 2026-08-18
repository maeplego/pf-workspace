package realtime

import "sync"

type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(channelID string) chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[channelID] == nil {
		h.rooms[channelID] = make(map[chan []byte]struct{})
	}
	h.rooms[channelID][ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(channelID string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.rooms[channelID]
	if subs == nil {
		return
	}
	delete(subs, ch)
	close(ch)
	if len(subs) == 0 {
		delete(h.rooms, channelID)
	}
}

func (h *Hub) Broadcast(channelID string, payload []byte) {
	h.mu.Lock()
	subs := h.rooms[channelID]
	targets := make([]chan []byte, 0, len(subs))
	for ch := range subs {
		targets = append(targets, ch)
	}
	h.mu.Unlock()
	for _, ch := range targets {
		select {
		case ch <- payload:
		default:
		}
	}
}
