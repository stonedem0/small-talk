package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)
var upgrader = websocket.Upgrader{}
var port = flag.String("port", "80", "provide port number")

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
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
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			fmt.Println(msg)
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
