import { useState, useEffect } from "react";
import { Routes, Route, useLocation, useNavigate } from "react-router-dom";
import Popup from "./Login/Login";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import DMChat from "./Chat/DMChat";
import Rules from "./Rules/Rules";
import Window from "./components/Window";
import { API_URL } from "./config";
import coinSound from "./assets/sounds/pickupCoin.wav";
import "./App.css";

const notifAudio = new Audio(coinSound);

const App = () => {
  
  const [username, setUsername] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [tab, setTab] = useState("Chat");
  const [windowClosed, setWindowClosed] = useState(false);
  const [notifications, setNotifications] = useState<{ [from: string]: { room: string; count: number } }>(() => {
    try { return JSON.parse(localStorage.getItem("dm_notifications") ?? "{}"); } catch { return {}; }
  });
  const [friendRequests, setFriendRequests] = useState<string[]>([]);
  const [friendAcceptedToast, setFriendAcceptedToast] = useState<string | null>(null);
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
    localStorage.setItem("dm_notifications", JSON.stringify(notifications));
  }, [notifications]);

  const unreadDMs = Object.fromEntries(Object.entries(notifications).map(([k, v]) => [k, v.count]));
  const clearDMNotif = (from: string) =>
    setNotifications((prev) => { const next = { ...prev }; delete next[from]; return next; });

  const handleSignOut = () => {
    localStorage.removeItem("username");
    localStorage.removeItem("token");
    localStorage.removeItem("dm_notifications");
    setUsername(null);
    setToken(null);
    setNotifications({});
    navigate("/");
  };

  useEffect(() => {
    window.addEventListener("auth:expired", handleSignOut);
    return () => window.removeEventListener("auth:expired", handleSignOut);
  }, []);

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
          const room: string = msg.room;
          setNotifications((prev) => ({
            ...prev,
            [from]: { room, count: (prev[from]?.count ?? 0) + 1 },
          }));
          notifAudio.currentTime = 0;
          notifAudio.play().catch(() => {});
        } else if (msg.type === "friend_request") {
          setFriendRequests((prev) => prev.includes(msg.from) ? prev : [...prev, msg.from]);
        } else if (msg.type === "friend_accepted") {
          setFriendAcceptedToast(`${msg.from} accepted your friend request! ♥`);
          setTimeout(() => setFriendAcceptedToast(null), 4000);
        }
      } catch {
        // ignore malformed events
      }
    };
    return () => es.close();
  }, [token]);

  const acceptFriend = async (from: string) => {
    await fetch(`${API_URL}/friends/accept`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ from }),
    });
    setFriendRequests((prev) => prev.filter((r) => r !== from));
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
          height={400}
          username={username}
        >
          <Popup setUsername={setUsername} setToken={setToken} />
        </Window>
      )}

      {token && !windowClosed && (
        <Window
          title="Fella connect"
          width={710}
          height={400}
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
              <Route path="/" element={<Rooms unreadDMs={unreadDMs} onDMOpen={clearDMNotif} />} />
              <Route path="/home" element={<Rooms unreadDMs={unreadDMs} onDMOpen={clearDMNotif} />} />
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

      {(Object.keys(notifications).length > 0 || friendRequests.length > 0) && (
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
          {Object.entries(notifications).map(([from, { count }]) => (
            <div key={from} className="notification-toast" onClick={() => {
              setNotifications((prev) => { const next = { ...prev }; delete next[from]; return next; });
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
