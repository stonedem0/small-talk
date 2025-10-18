package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var (
	ctx       = context.Background()
	jwtSecret []byte
	port      = getenv("PORT", "8080")

	onlineUsers     = make(map[string]map[string]bool) // room -> username -> online
	onlineUsersLock sync.Mutex

	subscriptions = make(map[string]bool)
	subLock       sync.Mutex
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// --- Pump-based WS architecture ---
// Each client has a dedicated readPump (socket->app) and writePump (app->socket).
// The server never does network I/O while holding global locks; it only enqueues
// bytes into a per-client buffered channel. Slow clients are isolated.

type client struct {
	conn     *websocket.Conn
	send     chan []byte
	room     string
	username string
}

var (
	rooms     = make(map[string]map[*client]struct{})
	roomsLock sync.RWMutex
)

// TODO: Lock this down later
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Message struct {
	Room      string `json:"room"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Type      string `json:"type,omitempty"`
	Timestamp string `json:"timestamp"`
}

func enqueueToRoom(room string, payload []byte) {
	roomsLock.RLock()
	set := rooms[room]
	for c := range set {
		select {
		case c.send <- payload:
		default:
			close(c.send)
			delete(set, c)
		}
	}
	roomsLock.RUnlock()
}

func writePump(c *client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func readPump(c *client) {
	defer func() {
		roomsLock.Lock()
		if set, ok := rooms[c.room]; ok {
			delete(set, c)
		}
		roomsLock.Unlock()
		close(c.send)
		c.conn.Close()

		onlineUsersLock.Lock()
		if onlineUsers[c.room] != nil {
			delete(onlineUsers[c.room], c.username)
		}
		onlineUsersLock.Unlock()

		leave := Message{Room: c.room, Username: c.username, Message: "left the room", Type: "system", Timestamp: time.Now().UTC().Format(time.RFC3339)}
		b, _ := json.Marshal(leave)
		RDB.Publish(ctx, "room:"+c.room, string(b))
		log.Printf("[Room %s] %s disconnected", c.room, c.username)
	}()

	c.conn.SetReadLimit(1 << 20)
	c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("json decode error: %v", err)
			continue
		}

		if msg.Type == "username_update" {
			oldU := msg.Username
			newU := msg.Message
			onlineUsersLock.Lock()
			if onlineUsers[c.room] != nil {
				delete(onlineUsers[c.room], oldU)
				onlineUsers[c.room][newU] = true
			}
			onlineUsersLock.Unlock()
			change := Message{Room: c.room, Username: oldU, Message: fmt.Sprintf("changed username to %s", newU), Type: "system", Timestamp: time.Now().UTC().Format(time.RFC3339)}
			b, _ := json.Marshal(change)
			RDB.Publish(ctx, "room:"+c.room, string(b))
			continue
		}

		msg.Room = c.room
		msg.Username = c.username
		msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
		b, _ := json.Marshal(msg)
		RDB.Publish(ctx, "room:"+c.room, string(b))
	}
}

func handleConnections(a *app, w http.ResponseWriter, r *http.Request) {
	if a.shutting.Load() {
		http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
		return
	}
	log.Printf("🔧 New WebSocket connection request")
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}

	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid token claims", http.StatusUnauthorized)
		return
	}
	if expRaw, ok := claims["exp"]; ok {
		switch exp := expRaw.(type) {
		case float64:
			if time.Now().After(time.Unix(int64(exp), 0)) {
				http.Error(w, "Token expired", http.StatusUnauthorized)
				return
			}
		}
	}
	username, ok := claims["username"].(string)
	if !ok || username == "" {
		http.Error(w, "Username missing in token", http.StatusUnauthorized)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade error: %v", err)
		return
	}

	c := &client{conn: ws, send: make(chan []byte, 64), room: room, username: username}
	roomsLock.Lock()
	if rooms[room] == nil {
		rooms[room] = make(map[*client]struct{})
	}
	rooms[room][c] = struct{}{}
	roomsLock.Unlock()

	onlineUsersLock.Lock()
	if onlineUsers[room] == nil {
		onlineUsers[room] = make(map[string]bool)
	}
	onlineUsers[room][username] = true
	onlineUsersLock.Unlock()

	go writePump(c)
	go readPump(c)

	subLock.Lock()
	if !subscriptions[room] {
		subscriptions[room] = true
		subLock.Unlock()
		go subscribeToRoom(room)
	} else {
		subLock.Unlock()
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		join := Message{Room: room, Username: username, Message: "joined the room", Type: "system", Timestamp: time.Now().UTC().Format(time.RFC3339)}
		b, _ := json.Marshal(join)
		RDB.LPush(ctx, "chat_history:"+room, b)
		RDB.LTrim(ctx, "chat_history:"+room, 0, 99)
		RDB.Publish(ctx, "room:"+room, string(b))
		log.Printf("[Room %s] join: %s", room, username)
	}()
}

func subscribeToRoom(room string) {
	pubsub := RDB.Subscribe(ctx, "room:"+room)
	defer pubsub.Close()
	ch := pubsub.Channel()

	for m := range ch {
		var received Message
		if err := json.Unmarshal([]byte(m.Payload), &received); err != nil {
			log.Printf("decode error: %v", err)
			continue
		}
		b, _ := json.Marshal(received)
		RDB.LPush(ctx, "chat_history:"+room, b)
		RDB.LTrim(ctx, "chat_history:"+room, 0, 99)

		enqueueToRoom(room, b)
	}
}

type app struct {
	server   *http.Server
	wg       sync.WaitGroup
	shutting atomic.Bool
	cancel   context.CancelFunc
}

func (a *app) gracefulShutdown(ctx context.Context) {
	a.cancel()
	a.wg.Wait()
	log.Println("Server stopped")
}

func main() {
	flag.Parse()
	InitRedis()

	root, cancel := context.WithCancel(context.Background())
	a := &app{
		server:   &http.Server{Addr: ":" + port},
		wg:       sync.WaitGroup{},
		shutting: atomic.Bool{},
		cancel:   cancel,
	}
	for _, room := range []string{"backrooms", "political", "overwatch is dead"} {
		RDB.SAdd(ctx, "rooms", room)
	}

	h := NewHandler(RDB, jwtSecret)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) { handleConnections(a, w, r) })
	http.HandleFunc("/login", WithCORS(h.LoginHandler))
	http.HandleFunc("/register", WithCORS(h.RegisterHandler))
	http.HandleFunc("/user-info", WithCORS(h.WithAuth(h.UserInfoHandler)))
	http.HandleFunc("/history", WithCORS(h.WithAuth(h.GetChatHistoryHandler)))
	http.HandleFunc("/rooms", WithCORS(h.WithAuth(h.GetRoomsHandler)))
	http.HandleFunc("/subscribe", WithCORS(h.WithAuth(h.SubscribeToRoomHandler)))
	http.HandleFunc("/online-users", WithCORS(h.WithAuth(h.GetOnlineUsersHandler)))
	http.HandleFunc("/room-usernames", WithCORS(h.GetRoomUsernamesHandler))
	http.HandleFunc("/create-room", WithCORS(h.WithAuth(h.CreateRoomHandler)))
	http.HandleFunc("/update-username", WithCORS(h.UpdateUsernameHandler))
	http.HandleFunc("/update-password", WithCORS(h.UpdatePasswordHandler))
	http.HandleFunc("/debug-users", WithCORS(h.DebugUsersHandler))

	addr := ":" + port
	log.Println("Server starting on", addr)

	go func() {
		if err := a.server.ListenAndServe(); err != nil {
			log.Fatal("ListenAndServe:", err)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	a.shutting.Store(true)
	shutdownCtx, cancel2 := context.WithTimeout(root, 15*time.Second)
	defer cancel2()
	a.gracefulShutdown(shutdownCtx)
	log.Println("Server stopped")
}
