import React, { useState } from "react";
import "./Popup.css";
import WindowControls from "../WindowControls/WindowControls";

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

  const handleMinimize = () => {
    console.log("Minimize clicked");
  };

  const handleFullscreen = () => {
    console.log("Fullscreen clicked");
    // if (!document.fullscreenElement) {
    //   document.documentElement.requestFullscreen().catch(err => {
    //     console.error(`Error attempting to enable full-screen mode: ${err.message} (${err.name})`);
    //   });
    // } else {
    //   document.exitFullscreen();
    // }
  };

  const handleClose = () => {
    console.log("Close clicked");
    // In this context, there's nothing to "close", so we can leave this blank
    // or maybe clear the input
    // setInput("");
  };

  return (
    <div id="popup-overlay">
      <div id="popup">
        <div className="popup-header">
          <span className="popup-title">Pick a username</span>
          <WindowControls
            onMinimize={handleMinimize}
            onFullscreen={handleFullscreen}
            onClose={handleClose}
          />
        </div>
        <div className="popup-body">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="username"
          />
          <button onClick={signIn}>Let me in</button>
        </div>
      </div>
    </div>
  );
};

export default Popup;
