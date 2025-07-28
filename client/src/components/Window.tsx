import { useNavigate } from "react-router-dom";
import { useState } from "react";
import WindowControls from "./WindowControls";
import "./Window.css";
import { API_URL } from "../config";

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
      navigate("/"); // ✅ fallback navigation
    }
  };

  const handleUsernameChange = async (e: React.FormEvent) => {
    e.preventDefault();
    console.log('🔧 handleUsernameChange called');
    console.log('🔧 Form submitted with newUsername:', newUsername);
    alert('Username change function called!');
    
    if (!newUsername.trim()) {
      console.log('🔧 Username is empty');
      alert('Username cannot be empty!');
      return;
    }
    
    console.log('🔧 Starting username change process...');
    
    setIsUpdating(true);
    const oldUsername = username;
    const newUsernameValue = newUsername.trim();
    
    console.log('🔧 Username change request:', { oldUsername, newUsername: newUsernameValue });
    
          try {
        console.log('🔧 Entering try block...');
        
        // Get current room from URL if we're in a chat room
        const pathParts = window.location.pathname.split('/');
        const currentRoom = pathParts[pathParts.length - 1];
        
        console.log('🔧 Current room from URL:', currentRoom);
        console.log('🔧 Full pathname:', window.location.pathname);
        console.log('🔧 Path parts:', pathParts);
      
                      // Always make HTTP request to update username in database, regardless of room
        console.log('🔧 Making username update request to server');
        
        const requestBody = {
          oldUsername: oldUsername,
          newUsername: newUsernameValue,
          room: currentRoom || 'home' // Use current room or 'home' as fallback
        };
        console.log('🔧 About to make HTTP request to update username:', requestBody);
        console.log('🔧 API_URL:', API_URL);
        console.log('🔧 Full URL will be:', `${API_URL}/update-username`);
        
        alert('About to make HTTP request to server...');
        
        const response = await fetch(`${API_URL}/update-username`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(requestBody),
        });
        
        console.log('🔧 HTTP response status:', response.status);
        console.log('🔧 HTTP response ok:', response.ok);
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error('🔧 Failed to update username on server:', errorText);
          alert('Failed to update username: ' + errorText);
          setIsUpdating(false);
          return;
        } else {
          const responseData = await response.json();
          console.log('🔧 Server response:', responseData);
          
          // Update localStorage with new username and token
          localStorage.setItem("username", responseData.newUsername);
          localStorage.setItem("token", responseData.token);
          
          console.log('🔧 Updated localStorage:', {
            username: responseData.newUsername,
            token: responseData.token ? '***' : 'missing'
          });
          
          // Try to send WebSocket message to update chat display (only if in a room)
          if (currentRoom && currentRoom !== 'home' && currentRoom !== '') {
            const ws = (window as any).currentWebSocket;
            if (ws && ws.readyState === WebSocket.OPEN) {
              console.log('🔧 Sending WebSocket username update message');
              const updateMessage = {
                type: "username_update",
                username: oldUsername,
                message: newUsernameValue
              };
              ws.send(JSON.stringify(updateMessage));
            }
          }
          
          // Close the form and show success
          setShowUsernameForm(false);
          setNewUsername("");
          
          console.log('🔧 Username update successful:', responseData);
          alert(`Username updated successfully!\nNew username: ${responseData.newUsername}\nPlease refresh the page manually.`);
          
          // Don't auto-reload, let user do it manually
        }
    } catch (error) {
      console.error('🔧 Error updating username:', error);
      alert('Error updating username: ' + error);
      setIsUpdating(false);
    }
  };

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault();
    console.log('🔧 handlePasswordChange called');
    
    if (!currentPassword.trim() || !newPassword.trim() || !confirmPassword.trim()) {
      console.log('🔧 Password fields are empty');
      return;
    }
    
    if (newPassword !== confirmPassword) {
      alert('New passwords do not match');
      return;
    }
    
    setIsUpdating(true);
    
    try {
      // Make HTTP request to update password
      const requestBody = {
        username: username,
        currentPassword: currentPassword.trim(),
        newPassword: newPassword.trim()
      };
      
      console.log('🔧 Making HTTP request to update password');
      
      const response = await fetch(`${API_URL}/update-password`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });
      
      console.log('🔧 HTTP response status:', response.status);
      
      if (!response.ok) {
        const errorText = await response.text();
        console.error('🔧 Failed to update password:', errorText);
        alert('Failed to update password: ' + errorText);
        setIsUpdating(false);
        return;
      } else {
        const responseData = await response.json();
        console.log('🔧 Password updated successfully');
        
        // Close the form
        setShowPasswordForm(false);
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
        alert('Password updated successfully!');
      }
    } catch (error) {
      console.error('🔧 Error updating password:', error);
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
              <button
                id="change-password"
                className="menu-button"
                title="Change password"
                onClick={() => setShowPasswordForm(true)}
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
