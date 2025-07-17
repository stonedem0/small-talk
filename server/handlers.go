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

func (h *Handler) WithAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle preflight
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusOK)
			return
		}

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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

func (h *Handler) GetOnlineUsersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
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
	}
	onlineUsersLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roomUsernames)
}

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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

	var creds Credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	username := creds.Username
	password := creds.Password

	if username == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username and password are required"})
		return
	}

	userJSON, err := RDB.HGet(ctx, "users", username).Result()
	if err != nil {
		if strings.Contains(err.Error(), "redis: nil") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		}
		return
	}

	var storedUser Credentials
	if err := json.Unmarshal([]byte(userJSON), &storedUser); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid user data"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(password))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid password"})
		return
	}
	w.WriteHeader(http.StatusOK)
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
