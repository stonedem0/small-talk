package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var (
	ctx            = context.Background()
	jwtSecret      []byte
	refreshSecret  []byte
	port           = getenv("PORT", "8080")
	debugEnabled   = getenv("DEBUG", "") == "true"
	allowedOrigins []string

	onlineUsers     = make(map[string]map[string]bool) // room -> username -> online
	onlineUsersLock sync.Mutex

	subscriptions = make(map[string]bool)
	subLock       sync.Mutex

	rooms     = make(map[string]map[*client]struct{})
	roomsLock sync.RWMutex

	roomSubs   = make(map[string]*redis.PubSub)
	roomSubsMu sync.Mutex
)

func init() {
	// Load .env if present; ok if missing (use system env in prod)
	_ = godotenv.Load()
	secret := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(secret) == "" {
		log.Fatal("JWT_SECRET is required; set it via environment or .env")
	}
	jwtSecret = []byte(secret)
	rsec := os.Getenv("REFRESH_JWT_SECRET")
	if strings.TrimSpace(rsec) == "" {
		log.Fatal("REFRESH_JWT_SECRET is required; set it via environment or .env")
	}
	refreshSecret = []byte(rsec)
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			o := strings.TrimSpace(p)
			if o != "" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
	}
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
	closed   atomic.Bool
}

type app struct {
	server   *http.Server
	wg       sync.WaitGroup
	shutting atomic.Bool
	cancel   context.CancelFunc
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		if len(allowedOrigins) == 0 {
			return false
		}
		origin := r.Header.Get("Origin")
		for _, o := range allowedOrigins {
			if origin == o {
				return true
			}
		}
		return false
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
	for c := range rooms[room] {
		select {
		case c.send <- payload:
		default:
			safeClose(c.send, &c.closed)
		}
	}
	roomsLock.RUnlock()

}

func safeClose(ch chan []byte, flag *atomic.Bool) {
	if flag.CompareAndSwap(false, true) {
		close(ch)
	}
}

// sendClose tries to initiate a WS close handshake cleanly.
func sendClose(conn *websocket.Conn) {
	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server_shutdown"),
		time.Now().Add(1*time.Second),
	)
}

func writePump(c *client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server_shutdown"),
					time.Now().Add(1*time.Second))
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
		roomsCounter.dec(c.room)
		safeClose(c.send, &c.closed)
		c.conn.Close()

		onlineUsersLock.Lock()
		if onlineUsers[c.room] != nil {
			delete(onlineUsers[c.room], c.username)
		}
		onlineUsersLock.Unlock()

		leave := Message{Room: c.room, Username: c.username, Message: "left the room", Type: "system", Timestamp: time.Now().UTC().Format(time.RFC3339)}
		b, _ := json.Marshal(leave)
		RDB.Publish(ctx, "room:"+c.room, string(b))
		if debugEnabled {
			log.Printf("[Room %s] %s disconnected", c.room, c.username)
		}
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
	if debugEnabled {
		log.Printf("🔧 New WebSocket connection request")
	}
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}

	var tokenStr string
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr = strings.TrimPrefix(auth, "Bearer ")
	}
	if tokenStr == "" {
		if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
			for _, p := range strings.Split(proto, ",") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "Bearer ") {
					tokenStr = strings.TrimPrefix(p, "Bearer ")
					break
				}
				if tokenStr == "" && p != "" {
					tokenStr = p
				}
			}
		}
	}
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
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

	// Echo back a selected subprotocol if the client sent one (required by browsers when specified)
	var respHeader http.Header
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		var chosen string
		parts := strings.Split(proto, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "Bearer ") {
				chosen = p
				break
			}
		}
		if chosen == "" && len(parts) > 0 {
			chosen = strings.TrimSpace(parts[0])
		}
		if chosen != "" {
			respHeader = http.Header{}
			respHeader.Set("Sec-WebSocket-Protocol", chosen)
		}
	}
	ws, err := upgrader.Upgrade(w, r, respHeader)
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

	roomsCounter.inc(room)

	onlineUsersLock.Lock()
	if onlineUsers[room] == nil {
		onlineUsers[room] = make(map[string]bool)
	}
	onlineUsers[room][username] = true
	onlineUsersLock.Unlock()

	a.wg.Add(2)
	go func() { defer a.wg.Done(); writePump(c) }()
	go func() { defer a.wg.Done(); readPump(c) }()
	subLock.Lock()
	if !subscriptions[room] {
		subscriptions[room] = true
		subLock.Unlock()
		go subscribeToRoom(ctx, room)
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
		if debugEnabled {
			log.Printf("[Room %s] join: %s", room, username)
		}
	}()
}

func subscribeToRoom(ctx context.Context, room string) {
	ps := RDB.Subscribe(ctx, "room:"+room)

	roomSubsMu.Lock()
	roomSubs[room] = ps
	roomSubsMu.Unlock()

	defer func() {
		roomSubsMu.Lock()
		delete(roomSubs, room)
		roomSubsMu.Unlock()
		ps.Close() // stop Redis subscription
		if debugEnabled {
			log.Printf("[Room %s] subscription stopped", room)
		}
	}()

	ch := ps.Channel()
	if debugEnabled {
		log.Printf("[Room %s] subscription started", room)
	}

	for {
		select {
		case m, ok := <-ch:
			if !ok {
				if debugEnabled {
					log.Printf("[Room %s] pubsub channel closed", room)
				}
				return
			}
			var received Message
			if err := json.Unmarshal([]byte(m.Payload), &received); err != nil {
				log.Printf("decode error in %s: %v", room, err)
				continue
			}
			b, _ := json.Marshal(received)
			if err := RDB.LPush(ctx, "chat_history:"+room, b).Err(); err != nil {
				log.Printf("redis LPush error in %s: %v", room, err)
			}
			_ = RDB.LTrim(ctx, "chat_history:"+room, 0, 99)

			enqueueToRoom(room, b)
		case <-ctx.Done():
			if debugEnabled {
				log.Printf("[Room %s] context canceled, stopping subscription", room)
			}
			return
		}
	}
}

func (a *app) gracefulShutdown(ctx context.Context) {
	log.Println("➜ graceful shutdown started")
	a.shutting.Store(true)
	drainingFlag.Store(true)
	sendHeartbeat()
	a.cancel()
	shutdownMsg := Message{Type: "system", Message: "server_shutdown", Timestamp: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.Marshal(shutdownMsg)
	roomsLock.RLock()
	for room := range rooms {
		enqueueToRoom(room, b)
	}
	roomsLock.RUnlock()

	roomSubsMu.Lock()
	for room, ps := range roomSubs {
		if debugEnabled {
			log.Printf("[Room %s] closing pubsub", room)
		}
		_ = ps.Close()
		delete(roomSubs, room)
	}
	roomSubsMu.Unlock()

	roomsLock.RLock()
	var toClose []*client
	for _, set := range rooms {
		for c := range set {
			toClose = append(toClose, c)
		}
	}
	roomsLock.RUnlock()
	for _, c := range toClose {
		safeClose(c.send, &c.closed)

	}

	done := make(chan struct{})
	go func() { a.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		for _, c := range toClose {
			_ = c.conn.Close()
		}
	}

	if err := a.server.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("http shutdown error: %v", err)
	}
}

func main() {
	flag.Parse()
	InitRedis()

	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	startHeartbeat(root)
	a := &app{
		server:   &http.Server{Addr: ":" + port},
		wg:       sync.WaitGroup{},
		shutting: atomic.Bool{},
		cancel:   cancel,
	}
	for _, room := range []string{"gaming", "music", "anime", "programming", "chilling", "nerd_herd", "pets", "emo"} {
		RDB.SAdd(ctx, "rooms", room)
	}

	h := NewHandler(RDB, jwtSecret, refreshSecret)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) { handleConnections(a, w, r) })
	http.HandleFunc("/login", WithCORS(h.LoginHandler))
	http.HandleFunc("/register", WithCORS(h.RegisterHandler))
	http.HandleFunc("/user-info", WithCORS(h.WithAuth(h.UserInfoHandler)))
	http.HandleFunc("/refresh", WithCORS(h.RefreshHandler))
	http.HandleFunc("/history", WithCORS(h.WithAuth(h.GetChatHistoryHandler)))
	http.HandleFunc("/rooms", WithCORS(h.WithAuth(h.GetRoomsHandler)))
	http.HandleFunc("/subscribe", WithCORS(h.WithAuth(h.SubscribeToRoomHandler)))
	http.HandleFunc("/online-users", WithCORS(h.WithAuth(h.GetOnlineUsersHandler)))
	http.HandleFunc("/room-usernames", WithCORS(h.GetRoomUsernamesHandler))
	http.HandleFunc("/create-room", WithCORS(h.WithAuth(h.CreateRoomHandler)))
	http.HandleFunc("/update-username", WithCORS(h.WithAuth(h.UpdateUsernameHandler)))
	http.HandleFunc("/update-password", WithCORS(h.WithAuth(h.UpdatePasswordHandler)))

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
