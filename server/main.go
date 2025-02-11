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
	clients["backrooms"] = make(map[*websocket.Conn]*sync.Mutex)
	clients["political"] = make(map[*websocket.Conn]*sync.Mutex)
	clients["overwatch is dead"] = make(map[*websocket.Conn]*sync.Mutex)
}

var ctx = context.Background()

var (
	clients       = make(map[string]map[*websocket.Conn]*sync.Mutex) // Map of rooms -> clients with mutex
	clientsLock   = sync.Mutex{}                                     // Protects access to the clients map
	subscriptions = make(map[string]bool)                            // Track active subscriptions per room
	subLock       = sync.Mutex{}
	port          = "8080"
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

func main() {
	flag.Parse()
	InitRedis()
	p := ":" + port
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/history", getChatHistoryHandler)
	http.HandleFunc("/rooms", getRoomsHandler)
	http.HandleFunc("/subscribe", subscribeToRoomHandler)
	log.Println("Server started on port", port)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
