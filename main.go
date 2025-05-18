package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/cors"
)

type BroadCaster struct {
	// Channel to broadcast messages
	broadcast chan []byte

	// Channel to accept new connections
	newConnection chan chan []byte

	// Channel to close connections
	closeConnection chan chan []byte

	// Connections
	connections map[chan []byte]int32
}

func (b *BroadCaster) Listen() {

	// ticker := time.Tick(6 * time.Second)
	var seq int32 = 0
	for {

		select {
		// broadcast message to all connections
		case message := <-b.broadcast:
			for connection := range b.connections {
				connection <- message
			}
		// add a new connection
		case connection := <-b.newConnection:
			b.connections[connection] = seq
			seq++
			log.Println("[NEW CONNECTION]")
		// close connection
		case connection := <-b.closeConnection:
			delete(b.connections, connection)
			log.Println("[CLIENT HAS GONE]")
		}
	}
}

func StartPinging(b *BroadCaster) {

	ticker := time.NewTicker(3 * time.Second)
	quit := make(chan struct{})

	go func() {

		for {
			select {
			case <-ticker.C:
				currentTime := strconv.FormatInt(time.Now().Unix(), 10)
				b.broadcast <- []byte(currentTime)

			case <-quit:
				ticker.Stop()
				return
			}

		}

	}()

}

type Server struct {
	broadcaster *BroadCaster
}

func NewBroadcaster() *BroadCaster {
	return &BroadCaster{
		broadcast:       make(chan []byte),
		newConnection:   make(chan chan []byte),
		closeConnection: make(chan chan []byte),
		connections:     make(map[chan []byte]int32),
	}
}

func NewServer() *Server {
	broadCaster := NewBroadcaster()
	server := &Server{
		broadCaster,
	}

	go broadCaster.Listen()
	return server
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fluser, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	setHeaders(w)

	clientConnection := make(chan []byte)
	server.broadcaster.newConnection <- clientConnection

	defer func() {
		server.broadcaster.closeConnection <- clientConnection
	}()

	isConnectionClose := r.Context().Done()

	go func() {
		<-isConnectionClose
		server.broadcaster.closeConnection <- clientConnection
	}()

	for {
		fmt.Fprintf(w, "data: %s\n\n", <-clientConnection)
		fluser.Flush()
	}
}

func main() {
	// Mux base
	mux := http.NewServeMux()

	// Middleware CORS
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "PUT", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler(mux)

	server := NewServer()

	StartPinging(server.broadcaster)

	mux.HandleFunc("/event", server.ServeHTTP)

	log.Println("Starting")
	log.Println("Error: ", http.ListenAndServe("localhost:9090", handler))
}
