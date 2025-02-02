import React, { useState } from "react";
import Chat from "./Chat/Chat";
import Rooms from "./Rooms/Rooms";
import Popup from "./Popup/Popup";
import "./App.css";

const App: React.FC = () => {
  const [username, setUsername] = useState<string>(localStorage.getItem("username") || "");

  return (
    <div id="main-container">
      {!username && <Popup setUsername={setUsername} />}
      {username && (
        <>
          <Chat username={username} />
          <Rooms />
        </>
      )}
    </div>
  );
};

export default App;
