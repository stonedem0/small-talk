import { useState, useEffect } from "react";
import { Routes, Route, useLocation, useNavigate } from "react-router-dom";
import Popup from "./Login/Login";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import Window from "./components/Window";
import "./App.css";

const App = () => {
  console.log("App component rendering");
  
  const [username, setUsername] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [tab, setTab] = useState("Chat");
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    console.log("App useEffect running");
    const storedUsername = localStorage.getItem("username");
    const storedToken = localStorage.getItem("token");
    console.log("Loading from localStorage:", { storedUsername, storedToken });
    
    if (storedToken) {
      setToken(storedToken);
      if (storedUsername) {
        setUsername(storedUsername);
      } else {
        console.log("Token exists but no username found - fetching from server");
        // Fetch username from server using the token
        fetch(`${import.meta.env.VITE_API_URL || 'http://localhost:8080'}/user-info`, {
          headers: {
            "Authorization": `Bearer ${storedToken}`
          }
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
          console.error("Failed to fetch username:", error);
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
    if (location.pathname === "/" || location.pathname === "/") {
      setTab("Chat");
    } else if (location.pathname.includes("/")) {
      setTab("Chat");
    }
  }, [location.pathname]);


  console.log(username, token);

  return (
      <div id="main-container">
        {!token && (
        <Window
          title="Login"
          width={300}
          height={200}
          username={username}
        >
          <Popup setUsername={setUsername} setToken={setToken} />
        </Window>
      )}

      {token && (
        <Window
          title="Fella connect"
          width={600}
          top="20%"
          left="50%"
          username={username}
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
            <div style={{ padding: "1rem" }}>
              <h2>File</h2>
            </div>
          )}
          {tab === "Chat" && (
            <Routes>
              <Route path="/" element={<Rooms />} />
              <Route path="/home" element={<Rooms />} />
              <Route path=":roomName" element={username ? <Chat username={username} /> : <div>Loading...</div>} />
            </Routes>
          )}
          {tab === "Settings" && (
            <div style={{ padding: "1rem" }}>
              <h2>Settings</h2>
            </div>
          )}
          {tab === "Appearance" && (
            <div style={{ padding: "1rem" }}>
              <h2>Appearance</h2>
  
            </div>
          )}
        </Window>
      )}
    </div>
  );
};

export default App;
