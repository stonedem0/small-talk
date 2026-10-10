// multinode-repro shows which features break when the API node (what the UI
// calls over REST/SSE) is not the node hosting the room.
//
// Needs: redis, postgres, the directory, and two app nodes:
//
//	node A  APP_ID=node-a PORT=8080 WS_PUBLIC_URL=ws://localhost:8080/ws
//	node B  APP_ID=node-b PORT=8082 WS_PUBLIC_URL=ws://localhost:8082/ws
//
// Rooms are pinned to node B through the directory's lease, then the checks use
// node A as the "API node". Each check has a control against node B itself.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	nodeA     = "http://localhost:8080"
	nodeB     = "http://localhost:8082"
	directory = "http://localhost:8081"
	origin    = "http://localhost:5174"
	wait      = 2500 * time.Millisecond
)

var failures int

func check(name string, ok bool, detail string) {
	status := "PASS"
	if !ok {
		status = "FAIL"
		failures++
	}
	fmt.Printf("  [%s] %s", status, name)
	if detail != "" {
		fmt.Printf("  (%s)", detail)
	}
	fmt.Println()
}

func req(method, url, token string, body any) (int, []byte) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, url, rd)
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		fmt.Println("request failed:", err, "- are both nodes and the directory running?")
		os.Exit(2)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func login(user string) string {
	req("POST", nodeA+"/register", "", map[string]string{"username": user, "password": "password123"})
	_, out := req("POST", nodeA+"/login", "", map[string]string{"username": user, "password": "password123"})
	var r struct{ Token string }
	_ = json.Unmarshal(out, &r)
	return r.Token
}

func pinRoomToNodeB(room string) {
	_ = exec.Command("redis-cli", "SET", "directory:room:"+room, "node-b", "EX", "300").Run()
}

func directoryURL(room, token string, dm bool, with string) string {
	u := directory + "/join?room=" + room
	if dm {
		u = directory + "/join?type=dm&with=" + with
	}
	code, out := req("GET", u, token, nil)
	var r struct {
		WSS string `json:"wss_url"`
	}
	_ = json.Unmarshal(out, &r)
	if code != 200 {
		fmt.Printf("directory join failed (%d): %s\n", code, out)
		os.Exit(2)
	}
	return r.WSS
}

type listener struct {
	mu   sync.Mutex
	msgs []string
}

func (l *listener) add(s string) { l.mu.Lock(); l.msgs = append(l.msgs, s); l.mu.Unlock() }
func (l *listener) count(substr string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, m := range l.msgs {
		if strings.Contains(m, substr) {
			n++
		}
	}
	return n
}

func (l *listener) saw(substr string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		for _, m := range l.msgs {
			if strings.Contains(m, substr) {
				l.mu.Unlock()
				return true
			}
		}
		l.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func dialWS(url, token string) (*websocket.Conn, *listener) {
	h := http.Header{}
	h.Set("Origin", origin)
	h.Set("Authorization", "Bearer "+token)
	c, resp, err := websocket.DefaultDialer.Dial(url, h)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		fmt.Printf("ws dial %s failed: %v (http %d)\n", url, err, code)
		os.Exit(2)
	}
	l := &listener{}
	go func() {
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			l.add(string(data))
		}
	}()
	return c, l
}

func openSSE(base, token string) *listener {
	l := &listener{}
	r, _ := http.NewRequest("GET", base+"/events?token="+token, nil)
	r.Header.Set("Origin", origin)
	resp, err := (&http.Client{}).Do(r)
	if err != nil {
		fmt.Println("sse failed:", err)
		os.Exit(2)
	}
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data:") {
				l.add(line)
			}
		}
	}()
	return l
}

func usersIn(base, room string) []string {
	_, out := req("GET", base+"/room-usernames", "", nil)
	var m map[string][]string
	_ = json.Unmarshal(out, &m)
	return m[room]
}

func count(base, room, token string) int {
	_, out := req("GET", base+"/online-users?room="+room, token, nil)
	var m map[string]int
	_ = json.Unmarshal(out, &m)
	return m["count"]
}

func has(list []string, want ...string) bool {
	set := map[string]bool{}
	for _, s := range list {
		set[s] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func main() {
	suffix := fmt.Sprint(time.Now().Unix() % 100000)
	alice, bob := "alice"+suffix, "bob"+suffix
	ta, tb := login(alice), login(bob)
	room := "repro" + suffix

	fmt.Println("Setup: room pinned to node B (:8082); node A (:8080) plays the UI's API node")
	pinRoomToNodeB(room)
	wss := directoryURL(room, ta, false, "")
	fmt.Println("  directory routes", room, "->", wss)
	if !strings.Contains(wss, ":8082") {
		fmt.Println("  room did not land on node B; is node B running with APP_ID=node-b?")
		os.Exit(2)
	}
	ca, _ := dialWS(wss, ta)
	cb, bobWS := dialWS(wss, tb)
	defer ca.Close()
	defer cb.Close()
	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n1. Online users in a room (sidebar: GET /room-usernames)")
	check("control: node B lists both users", has(usersIn(nodeB, room), alice, bob), fmt.Sprint(usersIn(nodeB, room)))
	check("API node A lists both users", has(usersIn(nodeA, room), alice, bob), fmt.Sprint(usersIn(nodeA, room)))

	fmt.Println("\n2. Online count for the room list (GET /online-users?room=)")
	check("control: node B counts 2", count(nodeB, room, ta) == 2, fmt.Sprint(count(nodeB, room, ta)))
	check("API node A counts 2", count(nodeA, room, ta) == 2, fmt.Sprint(count(nodeA, room, ta)))

	fmt.Println("\n3. Status change reaches the room (POST /status)")
	req("POST", nodeA+"/status", ta, map[string]string{"status": "via-A"})
	req("POST", nodeB+"/status", ta, map[string]string{"status": "via-B"})
	check("control: status posted to node B reaches the room", bobWS.saw("via-B", wait), "")
	check("status posted to API node A reaches the room", bobWS.saw("via-A", wait), "")

	fmt.Println("\n4. DM notification over SSE (GET /events)")
	dm := "dm:" + alice + ":" + bob
	if alice > bob {
		dm = "dm:" + bob + ":" + alice
	}
	pinRoomToNodeB(dm)
	dmURL := directoryURL("", ta, true, bob)
	if !strings.Contains(dmURL, ":8082") {
		fmt.Println("  DM room did not land on node B:", dmURL)
		os.Exit(2)
	}
	sseOnA, sseOnB := openSSE(nodeA, tb), openSSE(nodeB, tb)
	dmConn, _ := dialWS(dmURL, ta)
	defer dmConn.Close()
	time.Sleep(300 * time.Millisecond)
	_ = dmConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"chat","message":"hello bob"}`))
	check("control: bob's SSE on node B (the room's node) is notified", sseOnB.saw(`"type":"dm"`, wait), "")
	check("bob's SSE on API node A is notified", sseOnA.saw(`"type":"dm"`, wait), "")

	fmt.Println("\n5. Changing status inside a DM window must not ping the partner as a new DM")
	before := sseOnA.count(`"type":"dm"`)
	req("POST", nodeA+"/status", ta, map[string]string{"status": "just a status"})
	time.Sleep(1500 * time.Millisecond)
	extra := sseOnA.count(`"type":"dm"`) - before
	check("bob gets no DM notification for alice's status change", extra == 0, fmt.Sprintf("%d extra dm notification(s)", extra))

	fmt.Println()
	if failures > 0 {
		fmt.Printf("%d check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("all checks passed")
}
