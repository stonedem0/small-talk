package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Unread DM counts live in Redis, per user and per conversation partner
// (dm_unread:<user> is a hash of partner -> count), so they survive reloads and
// new devices and are counted while the user is offline.
//
// A message only counts as unread if the recipient is not in the DM room at that
// moment: the node hosting the room knows who is there, and someone looking at a
// conversation is reading it. Opening a DM clears its count.

func dmUnreadKey(username string) string { return "dm_unread:" + username }

// dmPartner returns the other participant of a DM room, or "" if room is not a DM
// room (or user is not in it).
func dmPartner(room, user string) string {
	if !isDMRoom(room) {
		return ""
	}
	parts := strings.SplitN(strings.TrimPrefix(room, "dm:"), ":", 2)
	if len(parts) != 2 {
		return ""
	}
	switch user {
	case parts[0]:
		return parts[1]
	case parts[1]:
		return parts[0]
	}
	return ""
}

// recordUnreadDM counts a new message from sender in the DM room for the recipient
// and tells the recipient's open sessions, on any node. Nothing is counted or sent
// when the recipient is looking at the conversation.
func recordUnreadDM(room, sender string) {
	recipient := dmPartner(room, sender)
	if recipient == "" {
		return
	}
	onlineUsersLock.Lock()
	reading := onlineUsers[room][recipient]
	onlineUsersLock.Unlock()
	if reading {
		return
	}
	unread, err := RDB.HIncrBy(ctx, dmUnreadKey(recipient), sender, 1).Result()
	if err != nil {
		log.Printf("dm unread increment failed for %s: %v", recipient, err)
		return
	}
	notif, _ := json.Marshal(map[string]interface{}{
		"type":   "dm",
		"from":   sender,
		"room":   room,
		"unread": unread,
	})
	pushNotification(recipient, string(notif))
}

// markDMRead clears the user's unread count for the conversation with partner and
// tells their other open sessions so badges clear live.
func markDMRead(username, partner string) {
	n, err := RDB.HDel(ctx, dmUnreadKey(username), partner).Result()
	if err != nil {
		log.Printf("dm unread clear failed for %s: %v", username, err)
		return
	}
	if n > 0 {
		notif, _ := json.Marshal(map[string]string{"type": "dm_read", "from": partner})
		pushNotification(username, string(notif))
	}
}

// GetDMUnreadHandler returns the caller's unread counts as {partner: count}.
func (h *Handler) GetDMUnreadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	username, err := h.VerifyToken(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}
	raw, err := h.RDB.HGetAll(r.Context(), dmUnreadKey(username)).Result()
	if err != nil {
		log.Printf("dm unread read failed for %s: %v", username, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	counts := make(map[string]int, len(raw))
	for partner, v := range raw {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			counts[partner] = n
		}
	}
	_ = json.NewEncoder(w).Encode(counts)
}
