package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

func init() {
	clients["general"] = make(map[*websocket.Conn]*sync.Mutex)
	clients["random"] = make(map[*websocket.Conn]*sync.Mutex)
	clients["gaming"] = make(map[*websocket.Conn]*sync.Mutex)
}

var ctx = context.Background()

var (
	clients       = make(map[string]map[*websocket.Conn]*sync.Mutex) // Map of rooms -> clients with mutex
	clientsLock   = sync.Mutex{}                                     // Protects access to the clients map
	subscriptions = make(map[string]bool)                            // Track active subscriptions per room
	subLock       = sync.Mutex{}
	port          = flag.String("port", "8080", "provide port number")
)

type Message struct {
	Room     string `json:"room"`
	Username string `json:"username"`
	Message  string `json:"message"`
	Colour   string `json:"colour"`
	Style    string `json:"style"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (for dev mode)
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS error: %v", err)
		return
	}

	clientsLock.Lock()
	if clients[room] == nil {
		clients[room] = make(map[*websocket.Conn]*sync.Mutex)
	}
	clients[room][ws] = &sync.Mutex{}
	clientsLock.Unlock()

	defer func() {
		clientsLock.Lock()
		ws.Close()
		delete(clients[room], ws)
		clientsLock.Unlock()
		log.Printf("Client disconnected from room: %s", room)
	}()

	// Ensure only one subscription per room
	subLock.Lock()
	if !subscriptions[room] {
		subscriptions[room] = true
		subLock.Unlock()
		go subscribeToRoom(room)
	} else {
		subLock.Unlock()
	}

	for {
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			log.Printf("Error reading JSON or client disconnected: %v", err)
			break
		}

		msg.Room = room // Ensure message is tagged with room
		msgBytes, _ := json.Marshal(msg)
		RDB.Publish(ctx, "room:"+room, string(msgBytes)) // Only publish, don't broadcast locally
	}
}

func subscribeToRoom(room string) {
	pubsub := RDB.Subscribe(ctx, "room:"+room)
	defer pubsub.Close()
	ch := pubsub.Channel()

	for msg := range ch {
		var receivedMsg Message
		if err := json.Unmarshal([]byte(msg.Payload), &receivedMsg); err != nil {
			log.Printf("Error decoding message: %v", err)
			continue
		}

		// Save message to history
		msgBytes, _ := json.Marshal(receivedMsg)
		RDB.LPush(ctx, "chat_history:"+room, msgBytes)
		RDB.LTrim(ctx, "chat_history:"+room, 0, 99) // Keep last 100 messages

		clientsLock.Lock()
		if _, exists := clients[receivedMsg.Room]; exists {
			for client, mutex := range clients[receivedMsg.Room] {
				mutex.Lock()
				client.WriteJSON(receivedMsg)
				mutex.Unlock()
			}
		}
		clientsLock.Unlock()
	}
}

func getChatHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}

	messages, err := RDB.LRange(ctx, "chat_history:"+room, 0, 99).Result()
	if err != nil {
		log.Printf("Error fetching chat history from Redis: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var history []Message
	for _, msg := range messages {
		var m Message
		if err := json.Unmarshal([]byte(msg), &m); err != nil {
			log.Printf("JSON Unmarshal Error: %v", err)
			continue
		}
		history = append(history, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func getRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	clientsLock.Lock()
	rooms := make([]string, 0, len(clients))
	for room := range clients {
		rooms = append(rooms, room)
	}
	clientsLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

func main() {
	flag.Parse()
	InitRedis()
	p := ":" + *port
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/history", getChatHistory)
	http.HandleFunc("/rooms", getRooms)

	log.Println("Server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
