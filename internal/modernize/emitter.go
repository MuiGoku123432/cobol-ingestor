package modernize

import (
	"net/http"
	"sync"
)

// EventEmitter abstracts SSE vs Wails event delivery for chat/swarm streaming.
type EventEmitter interface {
	Emit(event string, data any)
}

// SSEEmitter implements EventEmitter over HTTP Server-Sent Events.
type SSEEmitter struct {
	W  http.ResponseWriter
	Mu *sync.Mutex // nil for single-goroutine usage (chat), non-nil for swarm
}

func (e *SSEEmitter) Emit(event string, data any) {
	if e.Mu != nil {
		e.Mu.Lock()
		defer e.Mu.Unlock()
	}
	SendSSEJSON(e.W, event, data)
}

// SetSSEHeaders sets the required headers for SSE streaming.
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}
