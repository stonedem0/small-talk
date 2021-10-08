package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)
var upgrader = websocket.Upgrader{}
var port = flag.String("port", "80", "provide port number")
var file, _ = os.OpenFile("history.json", os.O_APPEND|os.O_WRONLY, 0644)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

var history = map[int]Message{}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	index := 0
	clients[ws] = true
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		broadcast <- msg
		history[index] = msg
		index++
	}

	// writing history to json file WIP
	b, _ := json.MarshalIndent(history, "", " ")
	file.Write(b)
	file.Write([]byte(","))

}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)

			}
		}
	}

}

// func cleanHistory() {
// loop every minute
// lock history file
// history.Clean()
// cap entries to 500
// unlock history file
// }

// type History struct {
// 	enc json.Encoder
// 	mu  sync.Mutex
// }

// func (h *History) AddMessage(m *Message) error {
// 	return nil
// }

// func (h *History) Clean() error {
// 	return nil
// }

func handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		file, err := os.Open("history.json")
		if err != nil {
			log.Printf("ERROR: %v\n", err)
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
	go handleMessages()
	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
