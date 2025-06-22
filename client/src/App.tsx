import React, { useState, useEffect } from "react";
import { Routes, Route } from "react-router-dom";
import Popup from "./Popup/Popup";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import "./App.css";

const App: React.FC = () => {
  const [username, setUsername] = useState<string | null>(null);

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
  };

  return (
    <div id="main-container">
      {!username && <Popup setUsername={handleSetUsername} />}
      {username && (
        <>
          <div className="user-header">
            <span className="username">Welcome, {username}!</span>
            <button onClick={handleSignOut} className="sign-out-btn">
              Sign out
            </button>
          </div>
          <Routes>
            <Route path="/" element={<Rooms username={username} />} />
            <Route path="/:roomName" element={<Chat username={username} />} />
          </Routes>
        </>
      )}
    </div>
  );
};

export default App;
