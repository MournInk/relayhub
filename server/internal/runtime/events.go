package runtimeapp

import (
	"encoding/json"
	"sync"
)

type EventHub struct {
	mu   sync.RWMutex
	subs map[chan []byte]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{subs: map[chan []byte]struct{}{}}
}

func (h *EventHub) Subscribe() chan []byte {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *EventHub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	close(ch)
	h.mu.Unlock()
}

func (h *EventHub) Publish(topic string, payload any) {
	data, err := json.Marshal(map[string]any{
		"topic":   topic,
		"payload": payload,
	})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- data:
		default:
		}
	}
}
