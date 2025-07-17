import { useNavigate } from "react-router-dom";
import { useState } from "react";
import WindowControls from "./WindowControls";
import "./Window.css";

type WindowProps = {
  title: string;
  children: React.ReactNode;
  width?: number;
  height?: number;
  top?: string;
  left?: string;
  username?: string | null;
  onSignOut?: () => void;
  onClose?: () => void; // ✅ NEW
  tabs?: string[];
  activeTab?: string;
  onTabClick?: (tab: string) => void;
};

const Window = ({
  title,
  children,
  width = 400,
  height = 200,
  top = "30%",
  left = "50%",
  username,
  onSignOut,
  onClose,
  tabs,
  activeTab,
  onTabClick,
}: WindowProps) => {
  const navigate = useNavigate();
  const [showUsernameForm, setShowUsernameForm] = useState(false);
  const [newUsername, setNewUsername] = useState("");

  const handleClose = () => {
    if (onClose) {
      onClose();
    } else {
      navigate("/"); // ✅ fallback navigation
    }
  };

  const handleUsernameChange = (e: React.FormEvent) => {
    e.preventDefault();
    if (newUsername.trim()) {
      localStorage.setItem("username", newUsername.trim());
      window.location.reload();
    }
  };

  const handleCancelUsernameChange = () => {
    setShowUsernameForm(false);
    setNewUsername("");
  };

  return (
    <div className="window" style={{ width, height, top, left }}>
      <div className="window-header">
        <span>{title}</span>
        <WindowControls />
      </div>

      {tabs && tabs.length > 0 && (
        <div className="window-tabs-container">
          <div className="window-tabs">
            {tabs.map((tab) => (
              <button
                key={tab}
                className={`tab ${
                  tab === activeTab || (!activeTab && tab === tabs[0])
                    ? "active"
                    : ""
                }`}
                onClick={() => onTabClick?.(tab)}
              >
                {tab.startsWith("_") ? tab : <u>{tab}</u>}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="window-content">
        {username && onSignOut && (
          <div className="window-menu-container">
            <div className="window-menu">
              <button
                id="leave-room"
                className="menu-button"
                title="Leave room"
                onClick={handleClose}
              ></button>
              <button
                id="change-username"
                className="menu-button"
                title="Change username"
                onClick={() => setShowUsernameForm(true)}
              ></button>
              <div className="sign-out">
                <span className="username">
                  oh hai, <strong>{username}</strong>!
                </span>
                <button onClick={onSignOut}>Sign out</button>
              </div>
            </div>
          </div>
        )}
        
        {showUsernameForm && (
          <div className="username-form-overlay">
            <div className="username-form">
              <h3>Change Username</h3>
              <form onSubmit={handleUsernameChange}>
                <input
                  type="text"
                  placeholder="Enter new username"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  autoFocus
                />
                <div className="form-buttons">
                  <button type="submit" disabled={!newUsername.trim()}>
                    Change
                  </button>
                  <button type="button" onClick={handleCancelUsernameChange}>
                    Cancel
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
        
        {children}
      </div>
    </div>
  );
};

export default Window;
