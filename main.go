package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"ssr-go/models"
	"sync"

	"github.com/go-chi/cors"
)

var (
	clients  = make(map[chan []byte]bool)
	clientMu sync.Mutex
)

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

	// Rutas
	mux.HandleFunc("/events", EventsHandler)
	mux.HandleFunc("/orders", OrdersHandler)

	// Inicia servidor con CORS
	log.Println("Server is working on port :9090")
	if err := http.ListenAndServe(":9090", handler); err != nil {
		log.Fatalf("Unable to start server %s", err.Error())
	}

}

func OrdersHandler(w http.ResponseWriter, r *http.Request) {
	var placed_order models.Order

	if err := json.NewDecoder(r.Body).Decode(&placed_order); err != nil {
		http.Error(w, "Error recibiendo los datos", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	log.Println("[OrderHandler] Orden recibida:", placed_order)

	msgBytes, err := json.Marshal(placed_order)
	if err != nil {
		http.Error(w, "Error serializando orden", http.StatusInternalServerError)
		return
	}

	for ch := range clients {
		select {
		case ch <- msgBytes:
		default:
			log.Println("[WARNING] cliente no responde")
		}
	}

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, "Orden recibida OK")
}

func EventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming no soportado", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan []byte)
	clientMu.Lock()
	clients[messageChan] = true
	clientMu.Unlock()

	defer func() {
		clientMu.Lock()
		delete(clients, messageChan)
		clientMu.Unlock()
		close(messageChan)
	}()

	// lectura de eventos para un cliente especifico
	for {
		msg, ok := <-messageChan
		if !ok {
			break
		}
		fmt.Fprintf(w, "data: %s\n\n", msg)
		flusher.Flush()
	}

}
