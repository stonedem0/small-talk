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

  return (
    <div id="main-container">
      {!username && <Popup setUsername={handleSetUsername} />}
      {username && (
        <Routes>
          {/* Home Page with Room List */}
          <Route path="/" element={<Rooms username={username} />} />

          {/* Dynamic Route for Chat Room */}
          <Route path="/:roomName" element={<Chat username={username} />} />
        </Routes>
      )}
    </div>
  );
};

export default App;
