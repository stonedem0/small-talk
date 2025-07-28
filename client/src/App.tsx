import { useState, useEffect } from "react";
import { Routes, Route, useLocation, useNavigate } from "react-router-dom";
import Popup from "./Login/Login";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import Window from "./components/Window";
import "./App.css";

const App = () => {
  const [username, setUsername] = useState<string | null>(null);
  const [tab, setTab] = useState("Chat");
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    const storedUsername = localStorage.getItem("username");
    if (storedUsername) {
      setUsername(storedUsername);
    }
  }, []);


  const handleSignOut = () => {
    localStorage.removeItem("username");
    setUsername(null);
    navigate("/");
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
      {!username && (
        <Window
          title="Fella connect"
          width={300}
          height={200}
          username={username}
          onSignOut={handleSignOut}
        >
          <Popup setUsername={setUsername} />
        </Window>
      )}

      {username && (
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
              <Route path=":roomName" element={<Chat username={username} />} />
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
