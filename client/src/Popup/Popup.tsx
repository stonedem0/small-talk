import React, { useState } from "react";
import "./Popup.css";

interface PopupProps {
  setUsername: (username: string) => void;
}

const Popup: React.FC<PopupProps> = ({ setUsername }) => {
  const [input, setInput] = useState<string>("");

  const signIn = () => {
    if (input.trim()) {
      localStorage.setItem("username", input.trim());
      setUsername(input.trim());
    } else {
      alert("Please enter a valid username.");
    }
  };

  return (
    <div id="popup-overlay">
      <div id="popup">
        <h2>Pick a username</h2>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
        <br />
        <button onClick={signIn}>Let me in</button>
      </div>
    </div>
  );
};

export default Popup;
