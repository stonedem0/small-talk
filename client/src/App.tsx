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

  const handleSetUsername = (name: string) => {
    localStorage.setItem("username", name);
    setUsername(name);
  };

  const handleSignOut = () => {
    localStorage.removeItem("username");
    setUsername(null);
    navigate("/");
  };

  // Sync tab state with route (for room URLs)
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
          <Popup setUsername={handleSetUsername} />
        </Window>
      )}

      {username && (
        <Window
          title="Fella connect"
          width={600}
          top="25%"
          left="50%"
          username={username}
          onSignOut={handleSignOut}
          tabs={["Chat", "Appearance", "Settings"]}
          activeTab={tab}
          onTabClick={(selected) => {
            setTab(selected);

            // Optional: Reset routing for tabs that are not Chat
            if (selected !== "Chat") {
              navigate("/");
            }
          }}
        >
          {/* Tab-driven conditional content */}
          {tab === "Chat" && (
            <Routes>
              <Route path="/" element={<Rooms username={username} />} />
              <Route path="/:roomName" element={<Chat username={username} />} />
            </Routes>
          )}
          {tab === "Settings" && (
            <div style={{ padding: "1rem" }}>
              <h2>Settings</h2>
              <p>Coming soon...</p>
            </div>
          )}
          {tab === "Appearance" && (
            <div style={{ padding: "1rem" }}>
              <h2>Appearance</h2>
              <p>Change themes and styles</p>
            </div>
          )}
          {tab === "_General" && (
            <div style={{ padding: "1rem" }}>
              <h2>Welcome</h2>
              <p>This is a general info panel.</p>
            </div>
          )}
        </Window>
      )}
    </div>
  );
};

export default App;
