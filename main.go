package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/stonedem0/small-talk/history"
)

// ./cmd/small-talk/main.go - set stuff up, connect them together
// ./ - library   smalltalk.Message ...

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)
var upgrader = websocket.Upgrader{}
var port = flag.String("port", "80", "provide port number")
var mu sync.Mutex

var file, _ = os.OpenFile("history.json", os.O_WRONLY|os.O_APPEND|os.O_RDONLY, 0644)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

// Handelling WS confections on the infinity loop
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
		if err != nil {
			log.Printf("error while reading JSON: %v", err)
			break
		}
		broadcast <- msg
		err = json.NewEncoder(file).Encode(msg)
		if err != nil {
			log.Printf("error while encoding JSON: %v", err)
		}
		// history.SaveHistory(history.Message(msg))
	}
}

// Processing messages
func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			// err := SaveHistory(msg)
			if err != nil {
				log.Printf("error while writing JSON: %v", err)
				client.Close()
				delete(clients, client)
				break
			}
		}
	}

}

// Implanatation for scheduled job
func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}

func main() {
	p := ":" + *port
	fs := http.FileServer(http.Dir("./client"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/history", history.GetHistory)
	go doEvery(5*time.Second, history.ClearHistory)
	go handleMessages()
	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
