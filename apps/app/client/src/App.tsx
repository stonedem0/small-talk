import { useState, useEffect } from "react";
import { Routes, Route, useLocation, useNavigate } from "react-router-dom";
import Popup from "./Login/Login";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import DMChat from "./Chat/DMChat";
import Rules from "./Rules/Rules";
import Window from "./components/Window";
import "./App.css";

const App = () => {
  
  const [username, setUsername] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [tab, setTab] = useState("Chat");
  const [windowClosed, setWindowClosed] = useState(false);
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


  const handleSignOut = () => {
    localStorage.removeItem("username");
    localStorage.removeItem("token");
    setUsername(null);
    setToken(null);
    navigate("/");
  };

  useEffect(() => {
    window.addEventListener("auth:expired", handleSignOut);
    return () => window.removeEventListener("auth:expired", handleSignOut);
  }, []);

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
              <Route path="/" element={<Rooms />} />
              <Route path="/home" element={<Rooms />} />
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
    </div>
  );
};

export default App;
