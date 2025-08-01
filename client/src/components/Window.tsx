import { useNavigate } from "react-router-dom";
import { useState } from "react";
import WindowControls from "./WindowControls";
import "./Window.css";
import { API_URL } from "../config";
import PrimaryButton from "./PrimaryButton";

type WindowProps = {
  title: string;
  children: React.ReactNode;
  width?: number;
  height?: number;
  top?: string;
  left?: string;
  username?: string | null;
  onSignOut?: () => void;
  onClose?: () => void; 
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
  const [showPasswordForm, setShowPasswordForm] = useState(false);
  const [newUsername, setNewUsername] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isUpdating, setIsUpdating] = useState(false);

  const handleClose = () => {
    if (onClose) {
      onClose();
    } else {
      navigate("/"); 
    }
  };

    const handleUsernameChange = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!newUsername.trim()) {
      return;
    }
    
    setIsUpdating(true);
    const oldUsername = username;
    const newUsernameValue = newUsername.trim();
    
    try {
      const pathParts = window.location.pathname.split('/');
      const currentRoom = pathParts[pathParts.length - 1];
      
      const requestBody = {
        oldUsername: oldUsername,
        newUsername: newUsernameValue,
        room: currentRoom || 'home' 
      };
        
        const response = await fetch(`${API_URL}/update-username`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(requestBody),
        });
        
            if (!response.ok) {
            const errorText = await response.text();
            alert('Failed to update username: ' + errorText);
            setIsUpdating(false);
            return;
          } else {
            const responseData = await response.json();
            
            localStorage.setItem("username", responseData.newUsername);
            localStorage.setItem("token", responseData.token);
            
            if (currentRoom && currentRoom !== 'home' && currentRoom !== '') {
              const ws = (window as any).currentWebSocket;
              if (ws && ws.readyState === WebSocket.OPEN) {
                const updateMessage = {
                  type: "username_update",
                  username: oldUsername,
                  message: newUsernameValue
                };
                ws.send(JSON.stringify(updateMessage));
              }
            }
            
            setShowUsernameForm(false);
            setNewUsername("");
            
            alert(`Username updated successfully! Please refresh the page.`);
          }
    } catch (error) {
      alert('Error updating username: ' + error);
      setIsUpdating(false);
    }
  };

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!currentPassword.trim() || !newPassword.trim() || !confirmPassword.trim()) {
      return;
    }
    
    if (newPassword !== confirmPassword) {
      alert('New passwords do not match');
      return;
    }
    
    setIsUpdating(true);
    
    try {
      const requestBody = {
        username: username,
        currentPassword: currentPassword.trim(),
        newPassword: newPassword.trim()
      };
      
      const response = await fetch(`${API_URL}/update-password`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });
      
      if (!response.ok) {
        const errorText = await response.text();
        alert('Failed to update password: ' + errorText);
        setIsUpdating(false);
        return;
      } else {
        setShowPasswordForm(false);
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
        alert('Password updated successfully!');
      }
    } catch (error) {
      alert('Error updating password: ' + error);
      setIsUpdating(false);
    }
  };

  const handleCancelPasswordChange = () => {
    setShowPasswordForm(false);
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
  };

  const handleCancelUsernameChange = () => {
    setShowUsernameForm(false);
    setNewUsername("");
  };

  const onCreateRoom = async () => {
    const roomName = prompt("Enter room name:");
    if (!roomName) {
      return;
    }
    const response = await fetch(`${API_URL}/create-room`, {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${localStorage.getItem("token")}`
      },
      body: JSON.stringify({ room: roomName })
    });
    if (!response.ok) {
      console.error("Failed to create room");
      return;
    }
    alert("Room created successfully");
    window.location.reload();
  };
  return (
    <div className="window" style={{ width, height, top, left }}>
      <div className="window-header">
        <div className="window-header-top">
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

        {onSignOut && (
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
              <button
                id="change-password"
                className="menu-button"
                title="Change password"
                onClick={() => setShowPasswordForm(true)}
              ></button>
              <button
                id="create-room"
                className="menu-button"
                title="Create room"
                onClick={onCreateRoom}
              ></button>
              <button
                id="sign-out"
                className="menu-button"
                title="Sign out"
                onClick={onSignOut}
              ></button>
            </div>
          </div>
        )}
      </div>

      <div className="window-content">
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
                  <button type="submit" disabled={!newUsername.trim() || isUpdating}>
                    {isUpdating ? "Updating..." : "Change"}
                  </button>
                  <button type="button" onClick={handleCancelUsernameChange} disabled={isUpdating}>
                    Cancel
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
        
        {showPasswordForm && (
          <div className="username-form-overlay">
            <div className="username-form">
              <h3>Change Password</h3>
              <form onSubmit={handlePasswordChange}>
                <input
                  type="password"
                  placeholder="Enter your current password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  autoFocus
                />
                <input
                  type="password"
                  placeholder="Enter new password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                />
                <input
                  type="password"
                  placeholder="Confirm new password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                />
                <div className="form-buttons">
                  <button type="submit" disabled={!currentPassword.trim() || !newPassword.trim() || !confirmPassword.trim() || isUpdating}>
                    {isUpdating ? "Updating..." : "Change"}
                  </button>
                  <button type="button" onClick={handleCancelPasswordChange} disabled={isUpdating}>
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
