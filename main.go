package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	// "github.com/stonedem0/small-talk/history"
)

var ctx = context.Background()

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan Message)
	upgrader  = websocket.Upgrader{}
	port      = flag.String("port", "8080", "provide port number")
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
	Colour   string `json:"colour"`
	Style    string `json:"style"`
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

// implanatation for scheduled job
func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
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
		getHistoryRedis(w, r)
	})
	http.HandleFunc("/subscribe", SubscribeToRoomHandler)
	// go doEvery(5*time.Second, history.ClearHistory)
	go handleMessages()
	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func getHistoryRedis(w http.ResponseWriter, r *http.Request) {
	msgBytesArray, err := RDB.LRange(ctx, "chat_history", 0, 50).Result()
	if err != nil {
		http.Error(w, "Redis LRange error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	messages := make([]Message, 0, len(msgBytesArray))

	for _, msgStr := range msgBytesArray {
		var m Message
		if err := json.Unmarshal([]byte(msgStr), &m); err == nil {
			messages = append(messages, m)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(messages); err != nil {
		log.Printf("Response encode error: %v", err)
	}
}
