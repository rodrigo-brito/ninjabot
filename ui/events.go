package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// sseEvent is one message of the Server-Sent Events stream.
type sseEvent struct {
	name string
	data []byte
}

// broker fans out events to every connected SSE client. Slow clients that
// cannot keep up are dropped instead of blocking the bot.
type broker struct {
	mu          sync.RWMutex
	subscribers map[chan sseEvent]struct{}
}

func newBroker() *broker {
	return &broker{subscribers: make(map[chan sseEvent]struct{})}
}

func (b *broker) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 256)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan sseEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
}

func (b *broker) hasSubscribers() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers) > 0
}

func (b *broker) publish(name string, payload interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.subscribers) == 0 {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("ui: failed to encode %s event: %v", name, err)
		return
	}

	event := sseEvent{name: name, data: data}
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// client is too slow; it will be disconnected by the writer
		}
	}
}

// handleEvents streams candle and order events to the browser.
func (c *Chart) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ch := c.events.subscribe()
	defer c.events.unsubscribe(ch)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event := <-ch:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.name, event.data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
