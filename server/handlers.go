package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Handler struct {
	RDB       *redis.Client
	JWTSecret []byte
	Ctx       context.Context
}

func NewHandler(rdb *redis.Client, jwtSecret []byte) *Handler {
	return &Handler{
		RDB:       rdb,
		JWTSecret: jwtSecret,
		Ctx:       context.Background(),
	}
}

func WithCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func (h *Handler) WithAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := h.VerifyToken(r)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "username", username)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) GetRoomsHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get rooms from Redis
	rooms, err := h.RDB.SMembers(h.Ctx, "rooms").Result()
	if err != nil {
		log.Printf("Error fetching rooms from Redis: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// If no rooms in Redis, return empty array
	if len(rooms) == 0 {
		rooms = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

func (h *Handler) SubscribeToRoomHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Room parameter is required", http.StatusBadRequest)
		return
	}

	clientsLock.Lock()
	_, exists := clients[room]
	clientsLock.Unlock()

	if !exists {
		http.Error(w, "Room does not exist", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Subscribed successfully"))
}
func (h *Handler) GetChatHistoryHandler(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) GetOnlineUsersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	onlineUsersLock.Lock()
	userCounts := make(map[string]int)
	for room, users := range onlineUsers {
		userCounts[room] = len(users)
	}
	onlineUsersLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userCounts)
}

func (h *Handler) GetRoomUsernamesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔧 GetRoomUsernamesHandler called")

	if r.Method == "OPTIONS" {
		log.Printf("🔧 GetRoomUsernamesHandler: OPTIONS request")
		w.WriteHeader(http.StatusOK)
		return
	}

	onlineUsersLock.Lock()
	roomUsernames := make(map[string][]string)
	for room, users := range onlineUsers {
		usernames := make([]string, 0, len(users))
		for username := range users {
			usernames = append(usernames, username)
		}
		roomUsernames[room] = usernames
		log.Printf("🔧 GetRoomUsernamesHandler: Room %s has users: %v", room, usernames)
	}
	onlineUsersLock.Unlock()

	log.Printf("🔧 GetRoomUsernamesHandler: Returning all room usernames: %v", roomUsernames)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roomUsernames)
}

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	username := creds.Username
	password := creds.Password

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	fmt.Println("hash: ", string(hash))
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if username == "" || password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
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

	user := map[string]string{"username": username, "password": string(hash)}
	userJSON, err := json.Marshal(user)

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = RDB.HSet(ctx, "users", username, userJSON).Err()
	if err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User registered successfully"))
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔧 LoginHandler called with method: %s", r.Method)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		log.Printf("🔧 LoginHandler: Invalid method %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}

	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		log.Printf("🔧 LoginHandler: Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	username := creds.Username
	password := creds.Password

	log.Printf("🔧 LoginHandler: Login attempt for username: %s", username)

	if username == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username and password are required"})
		return
	}

	userJSON, err := RDB.HGet(ctx, "users", username).Result()
	if err != nil {
		log.Printf("🔧 LoginHandler: User lookup failed for %s: %v", username, err)
		if strings.Contains(err.Error(), "redis: nil") {
			log.Printf("🔧 LoginHandler: User not found: %s", username)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		} else {
			log.Printf("🔧 LoginHandler: Redis error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		}
		return
	}

	log.Printf("🔧 LoginHandler: User found in database: %s", username)

	var storedUser Credentials
	if err := json.Unmarshal([]byte(userJSON), &storedUser); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(password))
	if err != nil {
		log.Printf("🔧 LoginHandler: Invalid password for user: %s", username)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid password"})
		return
	}

	log.Printf("🔧 LoginHandler: Password verified for user: %s", username)

	claims := jwt.MapClaims{
		"username": username,
		"exp":      jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(h.JWTSecret)
	if err != nil {
		log.Println("LoginHandler: Failed to sign token")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful",
		"token":   signedToken,
	})
	log.Printf("🔧 LoginHandler: Login successful for user: %s", username)

}

func (h *Handler) CreateRoomHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}

	var req struct {
		Room string `json:"room"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	room := strings.TrimSpace(req.Room)
	if room == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Room name is required"})
		return
	}

	exists, err := h.RDB.SIsMember(h.Ctx, "rooms", room).Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if exists {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "Room already exists"})
		return
	}

	err = h.RDB.SAdd(h.Ctx, "rooms", room).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save room"})
		return
	}

	clientsLock.Lock()
	clients[room] = make(map[*websocket.Conn]*sync.Mutex)
	clientsLock.Unlock()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Room created successfully", "room": room})
}

func (h *Handler) UpdateUsernameHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("UpdateUsernameHandler called with method: %s", r.Method)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		log.Printf("UpdateUsernameHandler: OPTIONS request, returning OK")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		log.Printf("UpdateUsernameHandler: Invalid method %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}

	var req struct {
		OldUsername string `json:"oldUsername"`
		NewUsername string `json:"newUsername"`
		Room        string `json:"room"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("UpdateUsernameHandler: Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	log.Printf("UpdateUsernameHandler: Request - OldUsername: %s, NewUsername: %s, Room: %s", req.OldUsername, req.NewUsername, req.Room)

	if req.OldUsername == "" || req.NewUsername == "" || req.Room == "" {
		log.Printf("UpdateUsernameHandler: Missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Old username, new username, and room are required"})
		return
	}

	// Verify the user exists
	userJSON, err := RDB.HGet(ctx, "users", req.OldUsername).Result()
	if err != nil {
		log.Printf("UpdateUsernameHandler: User not found: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	var storedUser Credentials
	if err := json.Unmarshal([]byte(userJSON), &storedUser); err != nil {
		log.Printf("UpdateUsernameHandler: Invalid user data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}

	// Check if new username already exists
	exists, err := RDB.HExists(ctx, "users", req.NewUsername).Result()
	if err != nil {
		log.Printf("UpdateUsernameHandler: Error checking new username: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}
	if exists {
		log.Printf("UpdateUsernameHandler: New username already taken: %s", req.NewUsername)
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "New username already taken"})
		return
	}

	// Update user in database
	newUser := map[string]string{"username": req.NewUsername, "password": storedUser.Password}
	newUserJSON, err := json.Marshal(newUser)
	if err != nil {
		log.Printf("UpdateUsernameHandler: Error marshaling new user data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// Use Redis transaction to ensure atomicity
	pipe := RDB.Pipeline()
	pipe.HDel(ctx, "users", req.OldUsername)
	pipe.HSet(ctx, "users", req.NewUsername, newUserJSON)
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("UpdateUsernameHandler: Error updating user in database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
		return
	}

	// Generate new JWT token with new username
	claims := jwt.MapClaims{
		"username": req.NewUsername,
		"exp":      jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(h.JWTSecret)
	if err != nil {
		log.Printf("UpdateUsernameHandler: Failed to generate new token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
		return
	}

	// Update online users tracking
	log.Printf("UpdateUsernameHandler: Updating online users for room %s", req.Room)
	onlineUsersLock.Lock()
	if onlineUsers[req.Room] != nil {
		log.Printf("UpdateUsernameHandler: Before update - Online users in %s: %v", req.Room, onlineUsers[req.Room])
		// Remove old username
		delete(onlineUsers[req.Room], req.OldUsername)
		// Add new username
		onlineUsers[req.Room][req.NewUsername] = true
		log.Printf("UpdateUsernameHandler: After update - Online users in %s: %v", req.Room, onlineUsers[req.Room])
	} else {
		log.Printf("UpdateUsernameHandler: No online users map found for room %s", req.Room)
	}
	onlineUsersLock.Unlock()

	// Broadcast username change message
	changeMsg := Message{
		Room:      req.Room,
		Username:  req.OldUsername,
		Message:   fmt.Sprintf("changed username to %s", req.NewUsername),
		Type:      "system",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	msgBytes, _ := json.Marshal(changeMsg)
	RDB.Publish(ctx, "room:"+req.Room, string(msgBytes))
	log.Printf("UpdateUsernameHandler: Published username change message: %s", string(msgBytes))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":     "Username updated successfully",
		"token":       signedToken,
		"newUsername": req.NewUsername,
	})
	log.Printf("UpdateUsernameHandler: Successfully updated username from %s to %s in room %s", req.OldUsername, req.NewUsername, req.Room)
}

func (h *Handler) UpdatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("UpdatePasswordHandler called with method: %s", r.Method)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		log.Printf("UpdatePasswordHandler: OPTIONS request, returning OK")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		log.Printf("UpdatePasswordHandler: Invalid method %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}

	var req struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("UpdatePasswordHandler: Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	log.Printf("UpdatePasswordHandler: Request - Username: %s", req.Username)

	if req.Username == "" || req.CurrentPassword == "" || req.NewPassword == "" {
		log.Printf("UpdatePasswordHandler: Missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username, current password, and new password are required"})
		return
	}

	// Verify the user exists and current password is correct
	userJSON, err := RDB.HGet(ctx, "users", req.Username).Result()
	if err != nil {
		log.Printf("UpdatePasswordHandler: User not found: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	var storedUser Credentials
	if err := json.Unmarshal([]byte(userJSON), &storedUser); err != nil {
		log.Printf("UpdatePasswordHandler: Invalid user data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}

	// Verify current password
	err = bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(req.CurrentPassword))
	if err != nil {
		log.Printf("UpdatePasswordHandler: Invalid current password")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid current password"})
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("UpdatePasswordHandler: Error hashing new password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// Update user in database
	newUser := map[string]string{"username": req.Username, "password": string(newHash)}
	newUserJSON, err := json.Marshal(newUser)
	if err != nil {
		log.Printf("UpdatePasswordHandler: Error marshaling new user data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// Update user in database
	err = RDB.HSet(ctx, "users", req.Username, newUserJSON).Err()
	if err != nil {
		log.Printf("UpdatePasswordHandler: Error updating user in database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Password updated successfully"})
	log.Printf("UpdatePasswordHandler: Successfully updated password for user %s", req.Username)
}

func (h *Handler) VerifyToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("missing or invalid Authorization header")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return h.JWTSecret, nil
	})

	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return "", fmt.Errorf("username missing from token")
	}

	return username, nil
}
