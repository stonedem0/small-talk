package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// isDMRoom reports whether the room is a DM room.
func isDMRoom(room string) bool {
	return strings.HasPrefix(room, "dm:")
}

// isDMParticipant reports whether username is one of the two participants in a DM room.
func isDMParticipant(room, username string) bool {
	parts := strings.SplitN(strings.TrimPrefix(room, "dm:"), ":", 2)
	return len(parts) == 2 && (username == parts[0] || username == parts[1])
}

type contextKey string

const (
	usernameKey contextKey = "username"

	maxUsernameLen = 32
	maxRoomLen     = 64
)

// dummyHash is a real bcrypt hash used when a login username is not found,
// so CompareHashAndPassword always runs the full bcrypt work and the response
// time doesn't reveal whether a username exists.
var dummyHash []byte

func init() {
	var err error
	dummyHash, err = bcrypt.GenerateFromPassword([]byte("dummy-timing-password"), bcrypt.DefaultCost)
	if err != nil {
		panic("failed to generate dummy bcrypt hash: " + err.Error())
	}
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Handler struct {
	RDB           *redis.Client
	JWTSecret     []byte
	RefreshSecret []byte
	Ctx           context.Context
}

func NewHandler(rdb *redis.Client, jwtSecret []byte, refreshSecret []byte) *Handler {
	return &Handler{RDB: rdb, JWTSecret: jwtSecret, RefreshSecret: refreshSecret, Ctx: context.Background()}
}

func WithCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if len(allowedOrigins) == 0 {
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		} else {
			allowed := false
			for _, o := range allowedOrigins {
				if origin == o {
					allowed = true
					break
				}
			}
			if !allowed {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func (h *Handler) WithAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := h.VerifyToken(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: " + err.Error()})
			return
		}
		var exists bool
		if err := DB.QueryRowContext(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username,
		).Scan(&exists); err != nil {
			log.Printf("WithAuth: db error for user %q: %v", username, err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
		if !exists {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: user no longer exists"})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), usernameKey, username)))
	}
}

func (h *Handler) GetRoomsHandler(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.RDB.SMembers(h.Ctx, "rooms").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if rooms == nil {
		rooms = []string{}
	}
	_ = json.NewEncoder(w).Encode(rooms)
}

func (h *Handler) GetRoomsWithCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.RDB.SMembers(h.Ctx, "rooms").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	categoryMap, err := h.RDB.HGetAll(h.Ctx, "room:categories").Result()
	if err != nil {
		categoryMap = map[string]string{}
	}
	grouped := map[string][]string{}
	for _, room := range rooms {
		cat, ok := categoryMap[room]
		if !ok || cat == "" {
			cat = "general"
		}
		grouped[cat] = append(grouped[cat], room)
	}
	_ = json.NewEncoder(w).Encode(grouped)
}

// No longer checks in-memory clients map. Source of truth is Redis set "rooms".
func (h *Handler) SubscribeToRoomHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}
	exists, err := h.RDB.SIsMember(h.Ctx, "rooms", room).Result()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Room does not exist", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Subscribed successfully"))
}

func (h *Handler) GetChatHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}
	if isDMRoom(room) {
		username, err := h.VerifyToken(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}
		if !isDMParticipant(room, username) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden"})
			return
		}
	}
	messages, err := RDB.LRange(r.Context(), "chat_history:"+room, 0, 99).Result()
	if err != nil {
		// history error
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// LPUSH stores newest-first; reverse to oldest-first for UI rendering
	var history []Message
	for i := len(messages) - 1; i >= 0; i-- {
		var m Message
		if err := json.Unmarshal([]byte(messages[i]), &m); err != nil {
			continue
		}
		history = append(history, m)
	}
	_ = json.NewEncoder(w).Encode(history)
}

// fix this function to return the online users for a specific room
func (h *Handler) GetOnlineUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	room := r.URL.Query().Get("room")
	onlineUsersLock.Lock()
	if room != "" {
		count := 0
		if users, ok := onlineUsers[room]; ok {
			count = len(users)
		}
		onlineUsersLock.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]int{"count": count})
		return
	}
	userCounts := make(map[string]int)
	for rm, users := range onlineUsers {
		userCounts[rm] = len(users)
	}
	onlineUsersLock.Unlock()
	_ = json.NewEncoder(w).Encode(userCounts)
}

func (h *Handler) GetRoomUsernamesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	onlineUsersLock.Lock()
	roomUsernames := make(map[string][]string)
	for room, users := range onlineUsers {
		list := make([]string, 0, len(users))
		for u := range users {
			list = append(list, u)
		}
		roomUsernames[room] = list
	}
	onlineUsersLock.Unlock()
	_ = json.NewEncoder(w).Encode(roomUsernames)
}

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(creds.Username)
	password := creds.Password
	if username == "" || password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}
	if len(username) > maxUsernameLen {
		http.Error(w, fmt.Sprintf("Username must be %d characters or fewer", maxUsernameLen), http.StatusBadRequest)
		return
	}
	if len(password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	_, err = DB.ExecContext(r.Context(),
		`INSERT INTO users (username, password_hash) VALUES ($1, $2)`,
		username, string(hash),
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			http.Error(w, "Username already taken", http.StatusConflict)
		} else {
			http.Error(w, "Failed to save user", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte("User registered successfully"))
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	username := strings.TrimSpace(creds.Username)
	password := creds.Password
	if username == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username and password are required"})
		return
	}
	var storedHash []byte
	userErr := DB.QueryRowContext(r.Context(),
		`SELECT password_hash FROM users WHERE username = $1`, username,
	).Scan(&storedHash)
	isNotFound := errors.Is(userErr, sql.ErrNoRows)
	if userErr != nil {
		// Always run bcrypt to prevent timing-based username enumeration.
		storedHash = dummyHash
	}
	hashErr := bcrypt.CompareHashAndPassword(storedHash, []byte(password))
	if userErr != nil && !isNotFound {
		log.Printf("LoginHandler: db error for user %q: %v", username, userErr)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if isNotFound || hashErr != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}
	claims := jwt.MapClaims{"username": username, "exp": jwt.NewNumericDate(time.Now().Add(24 * time.Hour))}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(h.JWTSecret)
	if err != nil {
		// token signing error
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
		return
	}
	// issue refresh token cookie
	rclaims := jwt.MapClaims{"username": username, "exp": jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))}
	rtok := jwt.NewWithClaims(jwt.SigningMethodHS256, rclaims)
	rSigned, err := rtok.SignedString(h.RefreshSecret)
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    rSigned,
			Path:     "/",
			HttpOnly: true,
			Secure:   os.Getenv("ENV") != "development",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Login successful", "token": signed})
}

func (h *Handler) CreateRoomHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	var req struct {
		Room     string `json:"room"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	room := strings.TrimSpace(req.Room)
	if room == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Room name is required"})
		return
	}
	if len(room) > maxRoomLen {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Room name must be %d characters or fewer", maxRoomLen)})
		return
	}
	if strings.HasPrefix(room, "dm:") {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Room name cannot start with 'dm:'"})
		return
	}
	exists, err := h.RDB.SIsMember(h.Ctx, "rooms", room).Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if exists {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Room already exists"})
		return
	}
	if err := h.RDB.SAdd(h.Ctx, "rooms", room).Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save room"})
		return
	}
	category := strings.TrimSpace(req.Category)
	if category != "" {
		_ = h.RDB.HSet(h.Ctx, "room:categories", room, category)
	}
	// no in-memory map to init here; rooms are created lazily when a client joins
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Room created successfully", "room": room})
}

func (h *Handler) UpdateUsernameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	var req struct{ OldUsername, NewUsername, Room string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	req.OldUsername = strings.TrimSpace(req.OldUsername)
	req.NewUsername = strings.TrimSpace(req.NewUsername)
	// Ensure the caller is the account owner
	tokenUser, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	if tokenUser != req.OldUsername {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden"})
		return
	}
	if req.OldUsername == "" || req.NewUsername == "" || req.Room == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Old username, new username, and room are required"})
		return
	}
	if len(req.NewUsername) > maxUsernameLen {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Username must be %d characters or fewer", maxUsernameLen)})
		return
	}
	var storedHash string
	err = DB.QueryRowContext(r.Context(),
		`SELECT password_hash FROM users WHERE username = $1`, req.OldUsername,
	).Scan(&storedHash)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}
	_, err = DB.ExecContext(r.Context(),
		`UPDATE users SET username = $1 WHERE username = $2`, req.NewUsername, req.OldUsername,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "New username already taken"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
		}
		return
	}
	claims := jwt.MapClaims{"username": req.NewUsername, "exp": jwt.NewNumericDate(time.Now().Add(24 * time.Hour))}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(h.JWTSecret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
		return
	}
	onlineUsersLock.Lock()
	if onlineUsers[req.Room] != nil {
		delete(onlineUsers[req.Room], req.OldUsername)
		onlineUsers[req.Room][req.NewUsername] = true
	}
	onlineUsersLock.Unlock()
	change := Message{Room: req.Room, Username: req.OldUsername, Message: fmt.Sprintf("changed username to %s", req.NewUsername), Type: "system", Timestamp: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.Marshal(change)
	RDB.Publish(r.Context(), "room:"+req.Room, string(b))
	// rotate refresh cookie as well
	rclaims := jwt.MapClaims{"username": req.NewUsername, "exp": jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))}
	rtok := jwt.NewWithClaims(jwt.SigningMethodHS256, rclaims)
	if rSigned, err2 := rtok.SignedString(h.RefreshSecret); err2 == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    rSigned,
			Path:     "/",
			HttpOnly: true,
			Secure:   os.Getenv("ENV") != "development",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Username updated successfully", "token": signed, "newUsername": req.NewUsername})
}

func (h *Handler) UpdatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	var req struct{ Username, CurrentPassword, NewPassword string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	// Ensure the caller is the account owner
	tokenUser, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	if tokenUser != req.Username {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden"})
		return
	}
	if req.Username == "" || req.CurrentPassword == "" || req.NewPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username, current password, and new password are required"})
		return
	}
	if len(req.NewPassword) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 8 characters"})
		return
	}
	var storedHash string
	err = DB.QueryRowContext(r.Context(),
		`SELECT password_hash FROM users WHERE username = $1`, req.Username,
	).Scan(&storedHash)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.CurrentPassword)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if _, err := DB.ExecContext(r.Context(),
		`UPDATE users SET password_hash = $1 WHERE username = $2`, string(newHash), req.Username,
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Password updated successfully"})
}

// DebugUsersHandler removed

func (h *Handler) UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"username": username})
}

func (h *Handler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	c, err := r.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Missing refresh token"})
		return
	}
	token, err := jwt.Parse(c.Value, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.RefreshSecret, nil
	})
	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid refresh token"})
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid claims"})
		return
	}
	username, _ := claims["username"].(string)
	if username == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid claims"})
		return
	}
	// issue new access token
	aclaims := jwt.MapClaims{"username": username, "exp": jwt.NewNumericDate(time.Now().Add(24 * time.Hour))}
	atok := jwt.NewWithClaims(jwt.SigningMethodHS256, aclaims)
	signed, err := atok.SignedString(h.JWTSecret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to issue token"})
		return
	}
	// rotate refresh
	rclaims := jwt.MapClaims{"username": username, "exp": jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))}
	rtok := jwt.NewWithClaims(jwt.SigningMethodHS256, rclaims)
	if rSigned, err2 := rtok.SignedString(h.RefreshSecret); err2 == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    rSigned,
			Path:     "/",
			HttpOnly: true,
			Secure:   os.Getenv("ENV") != "development",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"token": signed})
}

// StartDMHandler registers a DM between the caller and a target user.
// POST /dm/start  { "target": "bob" }
// Returns { "room": "dm:alice:bob" }
func (h *Handler) StartDMHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Target) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "target is required"})
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == username {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cannot DM yourself"})
		return
	}
	var targetExists bool
	err = DB.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, target,
	).Scan(&targetExists)
	if err != nil || !targetExists {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}
	a, b := username, target
	if a > b {
		a, b = b, a
	}
	room := "dm:" + a + ":" + b
	pipe := h.RDB.Pipeline()
	pipe.SAdd(h.Ctx, "dms:"+username, target)
	pipe.SAdd(h.Ctx, "dms:"+target, username)
	if _, err := pipe.Exec(h.Ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"room": room})
}

// GetDMListHandler returns the list of DM partners for the current user.
// GET /dms
func (h *Handler) GetDMListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	partners, err := h.RDB.SMembers(h.Ctx, "dms:"+username).Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if len(partners) == 0 {
		_ = json.NewEncoder(w).Encode([]string{})
		return
	}

	// Filter out partners who no longer exist in the database.
	rows, err := DB.QueryContext(r.Context(),
		`SELECT username FROM users WHERE username = ANY($1)`, pq.Array(partners),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	defer rows.Close()
	existing := make(map[string]bool, len(partners))
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
		existing[u] = true
	}
	if err := rows.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	live := partners[:0]
	for _, p := range partners {
		if existing[p] {
			live = append(live, p)
		} else {
			// Clean up the stale entry so it doesn't resurface.
			h.RDB.SRem(h.Ctx, "dms:"+username, p)
		}
	}
	if live == nil {
		live = []string{}
	}
	_ = json.NewEncoder(w).Encode(live)
}

func (h *Handler) VerifyToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("missing or invalid Authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return h.JWTSecret, nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return "", fmt.Errorf("token expired")
		}
		return "", fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	if expRaw, ok := claims["exp"]; ok {
		if t, ok := expRaw.(float64); ok && time.Now().After(time.Unix(int64(t), 0)) {
			return "", fmt.Errorf("token expired")
		}
	}
	username, ok := claims["username"].(string)
	if !ok || username == "" {
		return "", fmt.Errorf("username missing from token")
	}
	return username, nil
}

// pushNotification sends a notification event to all SSE connections for a user.
func pushNotification(username, payload string) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	for ch := range sseClients[username] {
		select {
		case ch <- payload:
		default: // drop if channel full
		}
	}
}

// SSEHandler handles Server-Sent Events for real-time notifications.
// Auth via ?token= query param since EventSource doesn't support headers.
func (h *Handler) SSEHandler(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.JWTSecret, nil
	})
	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	username, _ := claims["username"].(string)
	if username == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 8)
	sseClientsMu.Lock()
	if sseClients[username] == nil {
		sseClients[username] = make(map[chan string]struct{})
	}
	sseClients[username][ch] = struct{}{}
	sseClientsMu.Unlock()

	defer func() {
		sseClientsMu.Lock()
		delete(sseClients[username], ch)
		if len(sseClients[username]) == 0 {
			delete(sseClients, username)
		}
		sseClientsMu.Unlock()
	}()

	for {
		select {
		case payload := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// SendFriendRequestHandler sends a friend request from the caller to target.
// POST /friends/request  { "target": "bob" }
func (h *Handler) SendFriendRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Target) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "target is required"})
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == username {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "cannot friend yourself"})
		return
	}
	var targetExists bool
	if err := DB.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, target,
	).Scan(&targetExists); err != nil || !targetExists {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}
	// already friends?
	var isFriend bool
	_ = DB.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM friends WHERE (user_a=$1 AND user_b=$2) OR (user_a=$2 AND user_b=$1))`,
		username, target,
	).Scan(&isFriend)
	if isFriend {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "already friends"})
		return
	}
	_, err = DB.ExecContext(r.Context(),
		`INSERT INTO friend_requests (from_username, to_username) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		username, target,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	payload, _ := json.Marshal(map[string]string{"type": "friend_request", "from": username})
	pushNotification(target, string(payload))
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "request sent"})
}

// AcceptFriendRequestHandler accepts a pending friend request from sender.
// POST /friends/accept  { "from": "alice" }
func (h *Handler) AcceptFriendRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	var req struct {
		From string `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.From) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "from is required"})
		return
	}
	from := strings.TrimSpace(req.From)
	tx, err := DB.BeginTx(r.Context(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(r.Context(),
		`DELETE FROM friend_requests WHERE from_username = $1 AND to_username = $2`, from, username,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no pending request from that user"})
		return
	}
	a, b := username, from
	if a > b {
		a, b = b, a
	}
	if _, err := tx.ExecContext(r.Context(),
		`INSERT INTO friends (user_a, user_b) VALUES ($1, $2) ON CONFLICT DO NOTHING`, a, b,
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	payload, _ := json.Marshal(map[string]string{"type": "friend_accepted", "from": username})
	pushNotification(from, string(payload))
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "friend added"})
}

// DeclineFriendRequestHandler declines a pending friend request.
// POST /friends/decline  { "from": "alice" }
func (h *Handler) DeclineFriendRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	var req struct {
		From string `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.From) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "from is required"})
		return
	}
	if _, err := DB.ExecContext(r.Context(),
		`DELETE FROM friend_requests WHERE from_username = $1 AND to_username = $2`,
		strings.TrimSpace(req.From), username,
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "request declined"})
}

// RemoveFriendHandler removes a mutual friendship.
// DELETE /friends/remove  { "target": "bob" }
func (h *Handler) RemoveFriendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Target) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "target is required"})
		return
	}
	target := strings.TrimSpace(req.Target)
	if _, err := DB.ExecContext(r.Context(),
		`DELETE FROM friends WHERE (user_a=$1 AND user_b=$2) OR (user_a=$2 AND user_b=$1)`,
		username, target,
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "friend removed"})
}

// GetFriendsHandler returns the caller's friends list.
// GET /friends
func (h *Handler) GetFriendsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	rows, err := DB.QueryContext(r.Context(),
		`SELECT CASE WHEN user_a=$1 THEN user_b ELSE user_a END
		 FROM friends WHERE user_a=$1 OR user_b=$1`, username,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	defer rows.Close()
	friends := []string{}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err == nil {
			friends = append(friends, f)
		}
	}
	_ = json.NewEncoder(w).Encode(friends)
}

// GetFriendRequestsHandler returns the caller's pending incoming friend requests.
// GET /friends/requests
func (h *Handler) GetFriendRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	rows, err := DB.QueryContext(r.Context(),
		`SELECT from_username FROM friend_requests WHERE to_username = $1`, username,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	defer rows.Close()
	requests := []string{}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err == nil {
			requests = append(requests, f)
		}
	}
	_ = json.NewEncoder(w).Encode(requests)
}
