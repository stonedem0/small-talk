package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	// var h History
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
		fmt.Println("message", msg)
		history[index] = msg
		index++
	}
	b, _ := json.MarshalIndent(history, "", " ")
	file.Write(b)

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

func main() {
	p := ":" + *port
	fs := http.FileServer(http.Dir("./client"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
