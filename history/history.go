package history

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

var mu sync.Mutex

/* Once in a while (every minute in this case) we will clear history up to 50
messages. It makes it easy to load and display*/
func ClearHistory(t time.Time) {
	// Tru hard to avid deadlocks ☠️
	mu.Lock()
	file, err := os.Open("history.json")
	var count int
	if err != nil {
		log.Printf("error while opening history.json: %v", err)
	}
	dec := json.NewDecoder(file)
	for {
		var m Message
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			log.Printf("error while decoding history.json: %v", err)
		}
		count++
	}
	if count > 5 {
		os.Truncate("history.json", 0)
	}
	defer mu.Unlock()
}

func SaveHistory(msg Message) {
	var file, _ = os.OpenFile("history.json", os.O_WRONLY|os.O_APPEND|os.O_RDONLY, 0644)
	// var msg Message
	err := json.NewEncoder(file).Encode(msg)
	if err != nil {
		log.Printf("error while encoding history: %v", err)
	}
}

/* To give new joiners some context, we will get chunk of chat history
(last 50 messages in this case) to display on connections */
func GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		file, err := os.Open("history.json")
		if err != nil {
			log.Printf("error while reading history: %v", err)
		}
		io.Copy(w, file)
	}

}

// Little helpful for perodic stuff
func DoEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}
