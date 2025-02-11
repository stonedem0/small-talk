package main

import "net/http"

func SubscribeToRoomHandler(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "room query parameter is required", http.StatusBadRequest)
		return
	}
	go SubscribeToRoom(room)
	w.Write([]byte("subscribed to room: " + room))
}
