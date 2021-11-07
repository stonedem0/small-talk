package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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
		err1 := json.NewEncoder(file).Encode(msg)
		if err1 != nil {
			log.Printf("error while encoding JSON: %v", err)
		}
	}
}

// Processing messages
func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error while writing JSON: %v", err)
				client.Close()
				delete(clients, client)
				break
			}
		}
	}

}

// Claning chat history for the redability and performance reasons
func cleanHistory(t time.Time) {
	mu.Lock()
	file, err := os.Open("history.json")
	var count int
	if err != nil {
		log.Printf("error while reading history: %v", err)
	}
	dec := json.NewDecoder(file)
	for {
		var m Message
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			log.Printf("error while decoding history: %v", err)
		}
		count++
	}
	if count > 10 {
		os.Truncate("history.json", 0)
	}
	defer mu.Unlock()
}

// Implanatation for scheduled job
func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}

// To give new joiners some context, we will save chunk of chat history to display on connections
func handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		file, err := os.Open("history.json")
		if err != nil {
			log.Printf("error while reading history: %v", err)
		}
		io.Copy(w, file)
	}

}

func main() {
	p := ":" + *port
	fs := http.FileServer(http.Dir("./client"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/history", handleHistory)
	go doEvery(5*time.Second, cleanHistory)
	go handleMessages()
	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
