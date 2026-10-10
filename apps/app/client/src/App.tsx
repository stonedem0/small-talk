import { useState, useEffect, useCallback } from "react";
import { Routes, Route, useLocation, useNavigate } from "react-router-dom";
import Popup from "./Login/Login";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import DMChat from "./Chat/DMChat";
import Rules from "./Rules/Rules";
import Window from "./components/Window";
import { API_URL } from "./config";
import { authFetch } from "./utils/authFetch";
import coinSound from "./assets/sounds/pickupCoin.wav";
import "./App.css";

const notifAudio = new Audio(coinSound);

const App = () => {
  
  const [username, setUsername] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [tab, setTab] = useState("Chat");
  const [windowClosed, setWindowClosed] = useState(false);
  // Unread DM counts per conversation partner. The server is the source of truth: it
  // counts a message only while the recipient is away, and clears it when the DM is opened.
  const [unreadDMs, setUnreadDMs] = useState<{ [from: string]: number }>({});
  const [friendRequests, setFriendRequests] = useState<string[]>([]);
  const [friendAcceptedToast, setFriendAcceptedToast] = useState<string | null>(null);
  const [friendsRevision, setFriendsRevision] = useState(0);
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    const storedUsername = localStorage.getItem("username");
    const storedToken = localStorage.getItem("token");
    
    if (storedToken) {
      setToken(storedToken);
      if (storedUsername) {
        setUsername(storedUsername);
      } else {
        // Fetch username from server using the token
        fetch(`${import.meta.env.VITE_API_URL || 'http://localhost:8080'}/user-info`, {
          headers: {
            "Authorization": `Bearer ${storedToken}`
          },
          credentials: 'include'
        })
        .then(response => {
          if (response.ok) {
            return response.json();
          }
          throw new Error('Failed to fetch user info');
        })
        .then(data => {
          if (data.username) {
            setUsername(data.username);
            localStorage.setItem("username", data.username);
          }
        })
        .catch(error => {
          console.error("user-info fetch error", error);
          // Failed to fetch username
          // If we can't fetch the username, clear the token and redirect to login
          localStorage.removeItem("token");
          setToken(null);
        });
      }
    }
  }, []);


  useEffect(() => {
    if (!token) return;
    localStorage.removeItem("dm_notifications"); // legacy browser-only counts
    authFetch(`${API_URL}/dms/unread`)
      .then((r) => (r.ok ? r.json() : {}))
      .then((counts: { [from: string]: number }) =>
        // keep any live increment that raced ahead of this response
        setUnreadDMs((prev) => {
          const merged = { ...counts };
          for (const [from, n] of Object.entries(prev)) merged[from] = Math.max(n, merged[from] ?? 0);
          return merged;
        }))
      .catch(() => {});
  }, [token]);

  const clearDMNotif = (from: string) =>
    setUnreadDMs((prev) => { const next = { ...prev }; delete next[from]; return next; });

  const handleSignOut = useCallback(() => {
    localStorage.removeItem("username");
    localStorage.removeItem("token");
    localStorage.removeItem("rooms_selected_chat");
    localStorage.removeItem("rooms_contacts_hidden");
    setUsername(null);
    setToken(null);
    setUnreadDMs({});
    navigate("/");
  }, [navigate]);

  useEffect(() => {
    window.addEventListener("auth:expired", handleSignOut);
    return () => window.removeEventListener("auth:expired", handleSignOut);
  }, [handleSignOut]);

  useEffect(() => {
    if (!token) return;
    fetch(`${API_URL}/friends/requests`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.ok ? r.json() : [])
      .then((list: string[]) => setFriendRequests(list))
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    if (!token) return;
    const es = new EventSource(`${API_URL}/events?token=${token}`);
    es.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === "dm") {
          const from: string = msg.from;
          setUnreadDMs((prev) => ({ ...prev, [from]: msg.unread ?? (prev[from] ?? 0) + 1 }));
          notifAudio.currentTime = 0;
          notifAudio.play().catch(() => {});
        } else if (msg.type === "dm_read") {
          // the conversation was opened (possibly in another tab or device)
          clearDMNotif(msg.from);
        } else if (msg.type === "friend_request") {
          setFriendRequests((prev) => prev.includes(msg.from) ? prev : [...prev, msg.from]);
        } else if (msg.type === "friend_accepted") {
          setFriendAcceptedToast(`${msg.from} accepted your friend request! ♥`);
          setTimeout(() => setFriendAcceptedToast(null), 4000);
          setFriendsRevision((v) => v + 1);
        }
      } catch {
        // ignore malformed events
      }
    };
    // Every page holds one of the browser's few connections per host for these events.
    // A page frozen in the back/forward cache would keep it, and enough of them stall
    // the app, so release it when the page is hidden. If the browser restores the page
    // from that cache its connections and timers are stale: start clean.
    const onPageHide = () => es.close();
    const onPageShow = (e: PageTransitionEvent) => {
      if (e.persisted) window.location.reload();
    };
    window.addEventListener("pagehide", onPageHide);
    window.addEventListener("pageshow", onPageShow);
    return () => {
      es.close();
      window.removeEventListener("pagehide", onPageHide);
      window.removeEventListener("pageshow", onPageShow);
    };
  }, [token]);

  const acceptFriend = async (from: string) => {
    await fetch(`${API_URL}/friends/accept`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ from }),
    });
    setFriendRequests((prev) => prev.filter((r) => r !== from));
    setFriendsRevision((v) => v + 1);
  };

  const declineFriend = async (from: string) => {
    await fetch(`${API_URL}/friends/decline`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ from }),
    });
    setFriendRequests((prev) => prev.filter((r) => r !== from));
  };

  useEffect(() => {
    if (location.pathname === "/" || location.pathname === "/") {
      setTab("Chat");
    } else if (location.pathname.includes("/")) {
      setTab("Chat");
    }
  }, [location.pathname]);



  return (
      <div id="main-container">
        {!token && (
        <Window
          title="Login"
          width={300}
          username={username}
        >
          <Popup setUsername={setUsername} setToken={setToken} />
        </Window>
      )}

      {token && !windowClosed && (
        <Window
          title="small talk"
          width={600}
          height={420}
          username={username}
          onClose={() => setWindowClosed(true)}
          onSignOut={handleSignOut}
          tabs={["File", "Chat", "Appearance", "Settings"]}
          activeTab={tab}
          onTabClick={(selected) => {
            setTab(selected);
            if (selected !== "Chat") {
              navigate("/");
            }
          }}
        >
          {tab === "File" && (
            <div className="tab-container">
              <div className="tab-body" style={{ padding: "1rem" }}>
                <h2>File</h2>
              </div>
            </div>
          )}
          {tab === "Chat" && (
            <Routes>
              <Route path="/" element={<Rooms unreadDMs={unreadDMs} onDMOpen={clearDMNotif} friendsRevision={friendsRevision} />} />
              <Route path="/home" element={<Rooms unreadDMs={unreadDMs} onDMOpen={clearDMNotif} friendsRevision={friendsRevision} />} />
              <Route path="/rules" element={<Rules />} />
              <Route path="dm/:targetUsername" element={username ? <DMChat username={username} /> : <div>Loading...</div>} />
              <Route path=":roomName" element={username ? <Chat username={username} /> : <div>Loading...</div>} />
            </Routes>
          )}
          {tab === "Settings" && (
            <div className="tab-container">
              <div className="tab-body" style={{ padding: "1rem" }}>
                <h2>Settings</h2>
              </div>
            </div>
          )}
          {tab === "Appearance" && (
            <div className="tab-container">
              <div className="tab-body" style={{ padding: "1rem" }}>
                <h2>Appearance</h2>
              </div>
            </div>
          )}
        </Window>
      )}

      {friendAcceptedToast && (
        <div className="notifications">
          <div className="notification-toast friend-accepted-toast">{friendAcceptedToast}</div>
        </div>
      )}

      {(Object.keys(unreadDMs).length > 0 || friendRequests.length > 0) && (
        <div className="notifications">
          {friendRequests.map((from) => (
            <div key={`fr-${from}`} className="notification-toast friend-request-toast">
              <strong>{from}</strong> wants to be friends
              <div className="friend-request-actions">
                <button className="fr-btn fr-accept" onClick={() => acceptFriend(from)}>✓ accept</button>
                <button className="fr-btn fr-decline" onClick={() => declineFriend(from)}>✕ decline</button>
              </div>
            </div>
          ))}
          {Object.entries(unreadDMs).map(([from, count]) => (
            <div key={from} className="notification-toast" onClick={() => {
              clearDMNotif(from);
              navigate(`/dm/${from}`);
            }}>
              <strong>{from}</strong> — {count} new {count === 1 ? "message" : "messages"}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default App;
