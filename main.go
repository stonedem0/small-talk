package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)
var upgrader = websocket.Upgrader{}
var port = flag.String("port", "80", "provide port number")

var file, _ = os.OpenFile("history.json", os.O_WRONLY|os.O_APPEND|os.O_RDONLY, 0644)

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
		err1 := json.NewEncoder(file).Encode(msg)
		if err1 != nil {
			log.Printf("error: %v", err)
		}
	}

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

func cleanHistory(t time.Time) {
	// TODO: add lock/unlock
	file, err := os.Open("history.json")
	var count int
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(file)
	for {
		var m Message
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		count++
		fmt.Printf("%s: %s\n", m.Username, m.Message)
	}
	fmt.Println(count)
	if count > 10 {
		os.Truncate("history.json", 0)
	}

}

func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}

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
	go doEvery(5*time.Second, cleanHistory)
	go handleMessages()
	log.Println("http server started on port", p)
	err := http.ListenAndServe(p, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
