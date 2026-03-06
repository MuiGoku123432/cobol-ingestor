package modernize

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SendSSE writes a Server-Sent Event to the response writer and flushes.
func SendSSE(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// SendSSEJSON writes a Server-Sent Event with JSON-encoded data.
func SendSSEJSON(w http.ResponseWriter, event string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		SendSSE(w, "error", err.Error())
		return
	}
	SendSSE(w, event, string(b))
}
