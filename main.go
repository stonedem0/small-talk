package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	// "github.com/stonedem0/small-talk/history"
)

var ctx = context.Background()

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan Message)
	// upgrader  = websocket.Upgrader{}
	port = flag.String("port", "8080", "provide port number")
)

type Message struct {
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

// handling WS confections on the infinity loop
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	clients[ws] = true
	if err != nil {
		log.Printf("WS error: %v", err)
	}
	defer ws.Close()
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		fmt.Printf("msg: %v\n", msg)
		if err != nil {
			log.Printf("error while reading JSON: %v", err)
			break
		}
		broadcast <- msg
		msgBytes, _ := json.Marshal(msg)
		if err := RDB.LPush(ctx, "chat_history", msgBytes).Err(); err != nil {
			log.Printf("Redis LPUSH error: %v", err)
		}
		if err := RDB.LTrim(ctx, "chat_history", 0, 99).Err(); err != nil {
			log.Printf("Redis LTRIM error: %v", err)
		}
	}

}

// processing messages
func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error while writing JSON: %v", err)
				client.Close()
				delete(clients, client)
				continue
			}
		}
	}

}

func main() {
	flag.Parse()
	InitRedis()
	p := ":" + *port
	fs := http.FileServer(http.Dir("./client"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		getChatHistory(w, r)
	})
	http.HandleFunc("/subscribe", SubscribeToRoomHandler)
	go handleMessages()
	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func getChatHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	messages, err := RDB.LRange(ctx, "chat_history", 0, 99).Result()
	if err != nil {
		log.Printf("❌ Error fetching chat history from Redis: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var history []Message
	for _, msg := range messages {
		var m Message
		if err := json.Unmarshal([]byte(msg), &m); err != nil {
			log.Printf("❌ JSON Unmarshal Error: %v", err)
			continue
		}
		history = append(history, m)
	}

	json.NewEncoder(w).Encode(history)
}
