package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var jwtSecret []byte

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

	// Initialize default rooms in memory
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
	log.Printf("🔧 New WebSocket connection request")
	spew.Dump(r)
	room := r.URL.Query().Get("room")
	if room == "" {
		log.Printf("🔧 Missing room parameter")
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		log.Printf("🔧 Missing username parameter")
		http.Error(w, "Username parameter is required", http.StatusBadRequest)
		return
	}

	log.Printf("🔧 WebSocket connection request for room: %s, username: %s", room, username)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("🔧 WS upgrade error: %v", err)
		return
	}

	log.Printf("🔧 WebSocket upgraded successfully for %s in room %s", username, room)

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
		log.Printf("🔧 Created new online users map for room %s", room)
	}
	onlineUsers[room][username] = true
	userAdded = true
	log.Printf("🔧 Added user %s to online users for room %s. Current online users: %v", username, room, onlineUsers[room])
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
		log.Printf("🔧 WebSocket connection closing for %s in room %s", username, room)
		clientsLock.Lock()
		ws.Close()
		delete(clients[room], ws)
		clientsLock.Unlock()
		if userAdded && username != "" {
			onlineUsersLock.Lock()
			if onlineUsers[room] != nil {
				log.Printf("🔧 Removing user %s from online users in room %s", username, room)
				delete(onlineUsers[room], username)
				log.Printf("🔧 Remaining online users in room %s: %v", room, onlineUsers[room])
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
				log.Printf("🔧 Published leave message: %s", string(msgBytes))
			}
			onlineUsersLock.Unlock()
		}
		log.Printf("🔧 Client %s disconnected from room: %s", username, room)
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

		log.Printf("[Room %s] Received message from %s: type=%s, message=%s", room, username, msg.Type, msg.Message)

		// Handle username update messages
		if msg.Type == "username_update" {
			oldUsername := msg.Username
			newUsername := msg.Message // Using message field to store new username

			log.Printf("[Room %s] Received username update: %s -> %s", room, oldUsername, newUsername)

			// Update online users tracking
			onlineUsersLock.Lock()
			if onlineUsers[room] != nil {
				log.Printf("[Room %s] Before update - Online users: %v", room, onlineUsers[room])
				delete(onlineUsers[room], oldUsername)
				onlineUsers[room][newUsername] = true
				log.Printf("[Room %s] After update - Online users: %v", room, onlineUsers[room])
			} else {
				log.Printf("[Room %s] No online users map found for room", room)
			}
			onlineUsersLock.Unlock()

			// Broadcast username change message
			changeMsg := Message{
				Room:      room,
				Username:  oldUsername,
				Message:   fmt.Sprintf("changed username to %s", newUsername),
				Type:      "system",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			msgBytes, _ := json.Marshal(changeMsg)
			RDB.Publish(ctx, "room:"+room, string(msgBytes))
			log.Printf("[Room %s] Broadcasted username change message: %s", room, string(msgBytes))
			continue
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

	defaultRooms := []string{"backrooms", "political", "overwatch is dead"}
	for _, room := range defaultRooms {
		RDB.SAdd(ctx, "rooms", room)
	}

	handler := NewHandler(RDB, jwtSecret)
	p := ":" + port
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/login", WithCORS(handler.LoginHandler))
	http.HandleFunc("/register", WithCORS(handler.RegisterHandler))
	http.HandleFunc("/history", WithCORS(handler.WithAuth(handler.GetChatHistoryHandler)))
	http.HandleFunc("/rooms", (handler.WithAuth(handler.GetRoomsHandler)))
	http.HandleFunc("/subscribe", WithCORS(handler.WithAuth(handler.SubscribeToRoomHandler)))
	http.HandleFunc("/online-users", WithCORS(handler.WithAuth(handler.GetOnlineUsersHandler)))
	http.HandleFunc("/room-usernames", WithCORS(handler.GetRoomUsernamesHandler))
	http.HandleFunc("/create-room", WithCORS(handler.WithAuth(handler.CreateRoomHandler)))
	http.HandleFunc("/update-username", WithCORS(handler.UpdateUsernameHandler))
	http.HandleFunc("/update-password", WithCORS(handler.UpdatePasswordHandler))
	http.HandleFunc("/debug-users", WithCORS(handler.DebugUsersHandler))
	log.Println("Server started on port", port)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
