package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	usernameKey contextKey = "username"
)

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
	messages, err := RDB.LRange(ctx, "chat_history:"+room, 0, 99).Result()
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
	if len(password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	exists, err := RDB.HExists(ctx, "users", username).Result()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Username already taken", http.StatusConflict)
		return
	}
	userJSON, _ := json.Marshal(map[string]string{"username": username, "password": string(hash)})
	if err := RDB.HSet(ctx, "users", username, userJSON).Err(); err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
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
	username := creds.Username
	password := creds.Password
	if username == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username and password are required"})
		return
	}
	userJSON, err := RDB.HGet(ctx, "users", username).Result()
	if err != nil {
		if strings.Contains(err.Error(), "redis: nil") {
			// Normalize to avoid user enumeration
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		}
		return
	}
	var stored Credentials
	if err := json.Unmarshal([]byte(userJSON), &stored); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)); err != nil {
		// Normalize to avoid user enumeration
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
			Secure:   false,
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
		Room string `json:"room"`
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
	userJSON, err := RDB.HGet(ctx, "users", req.OldUsername).Result()
	fmt.Println("userJSON>>>", userJSON, "err>>>", err, "req.OldUsername>>>", req.OldUsername)
	if err != nil {
		fmt.Println("error>>>", err)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}
	var stored Credentials
	if err := json.Unmarshal([]byte(userJSON), &stored); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}
	exists, err := RDB.HExists(ctx, "users", req.NewUsername).Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if exists {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "New username already taken"})
		return
	}
	newUserJSON, _ := json.Marshal(map[string]string{"username": req.NewUsername, "password": stored.Password})
	pipe := RDB.Pipeline()
	pipe.HDel(ctx, "users", req.OldUsername)
	pipe.HSet(ctx, "users", req.NewUsername, newUserJSON)
	if _, err := pipe.Exec(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
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
	RDB.Publish(ctx, "room:"+req.Room, string(b))
	// rotate refresh cookie as well
	rclaims := jwt.MapClaims{"username": req.NewUsername, "exp": jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))}
	rtok := jwt.NewWithClaims(jwt.SigningMethodHS256, rclaims)
	if rSigned, err2 := rtok.SignedString(h.RefreshSecret); err2 == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    rSigned,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
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
	userJSON, err := RDB.HGet(ctx, "users", req.Username).Result()
	if err != nil {
		// Normalize to avoid user enumeration
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}
	var stored Credentials
	if err := json.Unmarshal([]byte(userJSON), &stored); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(req.CurrentPassword)); err != nil {
		// Normalize to avoid user enumeration
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
	newUserJSON, _ := json.Marshal(map[string]string{"username": req.Username, "password": string(newHash)})
	if err := RDB.HSet(ctx, "users", req.Username, newUserJSON).Err(); err != nil {
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
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"token": signed})
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
