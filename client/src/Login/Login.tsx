import React, { useState } from "react";
import "./Login.css";
import WindowControls from "../WindowControls/WindowControls";
import PrimaryButton from "../components/PrimaryButton";
import logo from "../assets/fella.png"; 

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
  };

  const handleClose = () => {
    console.log("Close clicked");
  };

  return (
    <div id="login-overlay">
      <div id="login-container">
        <div className="login-header">
          <span className="login-title">Fella connect</span>
          <WindowControls
            onMinimize={handleMinimize}
            onFullscreen={handleFullscreen}
            onClose={handleClose}
          />
        </div>
        <div className="login-body">
          <div className="icon">
            <img src={logo} alt="logo" />
          </div>
          <div className="input-group">
            <label htmlFor="username-input" className="form-label">
              Username:
            </label>
            <input
              id="username-input"
              type="text"
              className="form-input"
              value={input}
              onChange={(e) => setInput(e.target.value)}
            />
          </div>
          <PrimaryButton onClick={signIn}>Log In</PrimaryButton>
        </div>
      </div>
    </div>
  );
};

export default Popup;
