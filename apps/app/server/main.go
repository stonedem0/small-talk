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

	sseClients   = make(map[string]map[chan string]struct{})
	sseClientsMu sync.Mutex
)

func init() {
	// Load .env if present; ok if missing (use system env in prod)
	if err := godotenv.Load("app.env"); err != nil {
		_ = godotenv.Load()
	}
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
	internalAPIKey = os.Getenv("INTERNAL_API_KEY")
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
			messagesTotal.WithLabelValues("username_update").Inc()
			continue
		}

		msg.Room = c.room
		msg.Username = c.username
		msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
		b, _ := json.Marshal(msg)
		RDB.Publish(ctx, "room:"+c.room, string(b))
		msgType := msg.Type
		if msgType == "" {
			msgType = "chat"
		}
		messagesTotal.WithLabelValues(msgType).Inc()
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

	// Defense-in-depth: DM rooms may only be joined by their two participants
	if isDMRoom(room) && !isDMParticipant(room, username) {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
		RDB.Publish(ctx, "room:"+room, string(b))
		if debugEnabled {
			log.Printf("[Room %s] join: %s", room, username)
		}
	}()
}

func notifyDMRecipient(room, sender string) {
	if !isDMRoom(room) {
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(room, "dm:"), ":", 2)
	if len(parts) != 2 {
		return
	}
	recipient := parts[0]
	if recipient == sender {
		recipient = parts[1]
	}
	notif, _ := json.Marshal(map[string]string{
		"type": "dm",
		"from": sender,
		"room": room,
	})
	pushNotification(recipient, string(notif))
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
		ps.Close()
		subLock.Lock()
		delete(subscriptions, room)
		subLock.Unlock()
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
			if received.Type != "typing" && received.Type != "stop_typing" {
				if err := RDB.LPush(ctx, "chat_history:"+room, b).Err(); err != nil {
					log.Printf("redis LPush error in %s: %v", room, err)
				}
				_ = RDB.LTrim(ctx, "chat_history:"+room, 0, 99)

				if received.Type != "system" {
					notifyDMRecipient(room, received.Username)
				}
			}

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

func seedRooms() {
	rooms := []string{
		"gaming", "music", "anime", "programming", "chilling", "nerd_herd", "pets", "emo",
		"movies", "books", "sports", "cooking", "travel", "art", "photography",
		"fitness", "finance", "science", "history", "languages",
		"memes", "fashion", "diy", "crypto", "food", "cars",
		"astrology", "mental_health", "dating", "random",
	}
	for _, room := range rooms {
		if err := RDB.SAdd(ctx, "rooms", room).Err(); err != nil {
			log.Printf("seedRooms: SAdd %q: %v", room, err)
		}
	}
	categories := map[string]string{
		"gaming":        "gaming",
		"nerd_herd":     "gaming",
		"memes":         "gaming",
		"music":         "music",
		"emo":           "music",
		"anime":         "anime & arts",
		"art":           "anime & arts",
		"photography":   "anime & arts",
		"programming":   "tech",
		"science":       "tech",
		"finance":       "tech",
		"crypto":        "tech",
		"chilling":      "chill",
		"pets":          "chill",
		"cooking":       "chill",
		"food":          "chill",
		"movies":        "movies & books",
		"books":         "movies & books",
		"history":       "movies & books",
		"sports":        "sports & fitness",
		"fitness":       "sports & fitness",
		"travel":        "lifestyle",
		"languages":     "lifestyle",
		"fashion":       "lifestyle",
		"diy":           "lifestyle",
		"cars":          "lifestyle",
		"astrology":     "vibes & wellness",
		"mental_health": "vibes & wellness",
		"dating":        "vibes & wellness",
		"random":        "random",
	}
	for room, cat := range categories {
		if err := RDB.HSet(ctx, "room:categories", room, cat).Err(); err != nil {
			log.Printf("seedRooms: HSet %q: %v", room, err)
		}
	}
}

func registerRoutes(a *app, h *Handler) {
	http.Handle("/metrics", metricsHandler())
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) { handleConnections(a, w, r) })
	http.HandleFunc("/ping", WithCORS(h.PingHandler))
	http.HandleFunc("/login", WithCORS(RateLimitAuth(h.LoginHandler)))
	http.HandleFunc("/register", WithCORS(RateLimitAuth(h.RegisterHandler)))
	http.HandleFunc("/user-info", WithCORS(h.WithAuth(h.UserInfoHandler)))
	http.HandleFunc("/refresh", WithCORS(RateLimitAuth(h.RefreshHandler)))
	http.HandleFunc("/history", WithCORS(h.WithAuth(h.GetChatHistoryHandler)))
	http.HandleFunc("/rooms", WithCORS(h.WithAuth(h.GetRoomsHandler)))
	http.HandleFunc("/rooms-with-categories", WithCORS(h.WithAuth(h.GetRoomsWithCategoriesHandler)))
	http.HandleFunc("/subscribe", WithCORS(h.WithAuth(h.SubscribeToRoomHandler)))
	http.HandleFunc("/online-users", WithCORS(h.WithAuth(h.GetOnlineUsersHandler)))
	http.HandleFunc("/room-usernames", WithCORS(h.GetRoomUsernamesHandler))
	http.HandleFunc("/create-room", WithCORS(h.WithAuth(h.CreateRoomHandler)))
	http.HandleFunc("/update-username", WithCORS(h.WithAuth(h.UpdateUsernameHandler)))
	http.HandleFunc("/update-password", WithCORS(h.WithAuth(h.UpdatePasswordHandler)))
	http.HandleFunc("/dm/start", WithCORS(h.StartDMHandler))
	http.HandleFunc("/dms", WithCORS(h.GetDMListHandler))
	http.HandleFunc("/events", WithCORS(h.SSEHandler))
	http.HandleFunc("/friends", WithCORS(h.WithAuth(h.GetFriendsHandler)))
	http.HandleFunc("/friends/requests", WithCORS(h.WithAuth(h.GetFriendRequestsHandler)))
	http.HandleFunc("/friends/sent", WithCORS(h.WithAuth(h.GetSentFriendRequestsHandler)))
	http.HandleFunc("/friends/request", WithCORS(h.SendFriendRequestHandler))
	http.HandleFunc("/friends/accept", WithCORS(h.AcceptFriendRequestHandler))
	http.HandleFunc("/friends/decline", WithCORS(h.DeclineFriendRequestHandler))
	http.HandleFunc("/friends/remove", WithCORS(h.RemoveFriendHandler))
	http.HandleFunc("/status", WithCORS(h.WithAuth(h.SetStatusHandler)))
	http.HandleFunc("/statuses", WithCORS(h.GetStatusesHandler))
	http.HandleFunc("/favorites", WithCORS(h.WithAuth(h.ToggleFavoriteHandler)))
	http.HandleFunc("/favorites/list", WithCORS(h.WithAuth(h.GetFavoritesHandler)))
}

func main() {
	flag.Parse()
	InitRedis()
	InitDB()

	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	startHeartbeat(root)
	a := &app{
		server:   &http.Server{Addr: ":" + port, Handler: instrumentedMux(http.DefaultServeMux)},
		wg:       sync.WaitGroup{},
		shutting: atomic.Bool{},
		cancel:   cancel,
	}

	seedRooms()

	h := NewHandler(RDB, jwtSecret, refreshSecret)
	registerRoutes(a, h)

	addr := ":" + port
	log.Println("Server starting on", addr)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
