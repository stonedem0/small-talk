package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool) 
var broadcast = make(chan Message) 
var upgrader = websocket.Upgrader{}
type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}


func main() {
	fs := http.FileServer(http.Dir("/client"))
	http.Handle("/", fs)
	// http.HandleFunc("/ws", handleConnections)
	// go handleMessages()
	log.Println("http server started on :8000")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
			log.Fatal("ListenAndServe: ", err)
	}
}
