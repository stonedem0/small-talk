package main

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

// Presence is mirrored into Redis so any node can answer "who is online".
//
// A room lives on exactly one node, so that node's onlineUsers map is the source
// of truth and Redis is a copy rebuilt from it: a set of usernames per room
// (presence:room:<room>) plus an index of rooms that have one (presence:rooms).
// The copy is rewritten whole rather than adjusted with counters, so it cannot
// drift, and it expires if the hosting node dies without cleaning up.
const (
	presenceTTL      = 30 * time.Second
	presenceRoomsKey = "presence:rooms"
)

func presenceKey(room string) string { return "presence:room:" + room }

// presenceMu serializes writes so two updates for the same room cannot land in
// Redis out of order.
var presenceMu sync.Mutex

// syncPresence rewrites the room's presence from this node's local state. An empty
// room is deleted. Call it when users join, leave or rename; it is the only path
// that deletes, so a node that no longer hosts a room never wipes the new host's.
func syncPresence(room string) {
	presenceMu.Lock()
	defer presenceMu.Unlock()
	writePresence(room)
}

func writePresence(room string) {
	onlineUsersLock.Lock()
	users := make([]interface{}, 0, len(onlineUsers[room]))
	for u := range onlineUsers[room] {
		users = append(users, u)
	}
	onlineUsersLock.Unlock()

	pipe := RDB.TxPipeline()
	pipe.Del(ctx, presenceKey(room))
	if len(users) == 0 {
		pipe.SRem(ctx, presenceRoomsKey, room)
	} else {
		pipe.SAdd(ctx, presenceKey(room), users...)
		pipe.Expire(ctx, presenceKey(room), presenceTTL)
		pipe.SAdd(ctx, presenceRoomsKey, room)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("presence sync error for %s: %v", room, err)
	}
}

// syncAllPresence refreshes the expiry and repairs the copy for every room that
// has users on this node. It never deletes, only the event path does.
func syncAllPresence() {
	onlineUsersLock.Lock()
	var active []string
	for room, users := range onlineUsers {
		if len(users) > 0 {
			active = append(active, room)
		}
	}
	onlineUsersLock.Unlock()

	presenceMu.Lock()
	defer presenceMu.Unlock()
	for _, room := range active {
		writePresence(room)
	}
}

// readPresence returns the online users of every room, across all nodes.
func readPresence(c context.Context) (map[string][]string, error) {
	rooms, err := RDB.SMembers(c, presenceRoomsKey).Result()
	if err != nil {
		return nil, err
	}
	pipe := RDB.Pipeline()
	cmds := make([]interface{ Result() ([]string, error) }, len(rooms))
	for i, room := range rooms {
		cmds[i] = pipe.SMembers(c, presenceKey(room))
	}
	if _, err := pipe.Exec(c); err != nil {
		return nil, err
	}

	out := make(map[string][]string, len(rooms))
	var stale []interface{}
	for i, room := range rooms {
		users, _ := cmds[i].Result()
		if len(users) == 0 {
			stale = append(stale, room) // key expired: its node is gone
			continue
		}
		sort.Strings(users)
		out[room] = users
	}
	if len(stale) > 0 {
		_ = RDB.SRem(c, presenceRoomsKey, stale...).Err()
	}
	return out, nil
}

// userRooms lists the rooms the user is currently in, on any node.
func userRooms(c context.Context, username string) ([]string, error) {
	all, err := readPresence(c)
	if err != nil {
		return nil, err
	}
	var rooms []string
	for room, users := range all {
		for _, u := range users {
			if u == username {
				rooms = append(rooms, room)
				break
			}
		}
	}
	sort.Strings(rooms)
	return rooms, nil
}
