package middlewares

import (
	"fmt"
	"net/http"
	"sync"
)

type SSEServer struct {
	Clients map[chan string]bool
	mu      sync.Mutex
}

func NewSSEServer() *SSEServer {
	return &SSEServer{
		Clients: make(map[chan string]bool),
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

	fmt.Printf("Sending notification: %s\n", message)

	for client := range s.Clients {
		client <- message
	}
}
