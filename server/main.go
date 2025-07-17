package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var jwtSecret []byte

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

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

var onlineUsers = make(map[string]map[string]bool) // room -> username -> online
var onlineUsersLock = sync.Mutex{}

type Message struct {
	Room      string `json:"room"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Type      string `json:"type,omitempty"`
	Timestamp string `json:"timestamp"`
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

	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username parameter is required", http.StatusBadRequest)
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

	userAdded := false

	// Add user to online users immediately upon connection
	onlineUsersLock.Lock()
	if onlineUsers[room] == nil {
		onlineUsers[room] = make(map[string]bool)
	}
	onlineUsers[room][username] = true
	userAdded = true
	onlineUsersLock.Unlock()

	// Broadcast join message with a small delay to ensure client is ready
	go func() {
		time.Sleep(100 * time.Millisecond) // Small delay to ensure client is ready
		joinMsg := Message{
			Room:      room,
			Username:  username,
			Message:   "joined the room",
			Type:      "system",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		msgBytes, _ := json.Marshal(joinMsg)

		// Save join message to Redis chat history
		RDB.LPush(ctx, "chat_history:"+room, msgBytes)
		RDB.LTrim(ctx, "chat_history:"+room, 0, 99) // Keep last 100 messages

		// Broadcast to all clients
		RDB.Publish(ctx, "room:"+room, string(msgBytes))
		log.Printf("[Room %s] Sent join message for user '%s'", room, username)
	}()

	defer func() {
		clientsLock.Lock()
		ws.Close()
		delete(clients[room], ws)
		clientsLock.Unlock()
		if userAdded && username != "" {
			onlineUsersLock.Lock()
			if onlineUsers[room] != nil {
				delete(onlineUsers[room], username)
				// Broadcast leave message
				leaveMsg := Message{
					Room:      room,
					Username:  username,
					Message:   "left the room",
					Type:      "system",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				}
				msgBytes, _ := json.Marshal(leaveMsg)
				RDB.Publish(ctx, "room:"+room, string(msgBytes))
			}
			onlineUsersLock.Unlock()
		}
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

		msg.Room = room
		msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
		msgBytes, _ := json.Marshal(msg)
		RDB.Publish(ctx, "room:"+room, string(msgBytes))
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
	handler := NewHandler(RDB, jwtSecret)
	p := ":" + port
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/login", handler.LoginHandler)
	http.HandleFunc("/register", handler.RegisterHandler)
	http.HandleFunc("/history", handler.WithAuth(handler.GetChatHistoryHandler))
	http.HandleFunc("/rooms", handler.WithAuth(handler.GetRoomsHandler))
	http.HandleFunc("/subscribe", handler.WithAuth(handler.SubscribeToRoomHandler))
	http.HandleFunc("/create-room", handler.WithAuth(handler.CreateRoomHandler))
	http.HandleFunc("/online-users", handler.WithAuth(handler.GetOnlineUsersHandler))
	http.HandleFunc("/room-usernames", handler.GetRoomUsernamesHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("UNMATCHED: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	log.Println("Server started on port", port)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// func getOnlineUsernames(room string) []string {
// 	users := []string{}
// 	for u := range onlineUsers[room] {
// 		users = append(users, u)
// 	}
// 	return users
// }
