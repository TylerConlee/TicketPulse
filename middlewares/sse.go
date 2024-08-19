package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type SSEServer struct {
	Clients          map[chan string]bool
	mu               sync.Mutex
	ConnectionStatus map[string]map[string]string // Store status of services
}

func NewSSEServer() *SSEServer {
	return &SSEServer{
		Clients:          make(map[chan string]bool),
		ConnectionStatus: make(map[string]map[string]string), // Initialize the connection status map
	}
}

func (s *SSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	notificationChan := make(chan string)
	s.mu.Lock()
	s.Clients[notificationChan] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.Clients, notificationChan)
		s.mu.Unlock()
		close(notificationChan)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial status
	s.mu.Lock()
	for service, statusData := range s.ConnectionStatus {
		statusMsg, _ := json.Marshal(map[string]interface{}{
			"event":   "connection-status",
			"service": service,
			"data":    statusData,
		})
		fmt.Fprintf(w, "data: %s\n\n", statusMsg)
		flusher.Flush()
	}
	s.mu.Unlock()

	for {
		select {
		case msg := <-notificationChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *SSEServer) NotifyAll(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var msgData map[string]interface{}
	if err := json.Unmarshal([]byte(message), &msgData); err == nil {
		if event, ok := msgData["event"].(string); ok && event == "connection-status" {
			data := msgData["data"].(map[string]interface{})
			service := data["service"].(string)

			// Convert interface{} map to map[string]string
			statusMap := make(map[string]string)
			for k, v := range data {
				statusMap[k] = fmt.Sprintf("%v", v)
			}

			s.ConnectionStatus[service] = statusMap // Store the latest status
		}
	}

	for client := range s.Clients {
		client <- message
	}
}
