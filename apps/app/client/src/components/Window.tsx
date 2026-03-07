import { useNavigate, useLocation } from "react-router-dom";
import { useState, useRef } from "react";
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
  const location = useLocation();
  const [showUsernameForm, setShowUsernameForm] = useState(false);
  const [showPasswordForm, setShowPasswordForm] = useState(false);
  const [newUsername, setNewUsername] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isUpdating, setIsUpdating] = useState(false);
  const [showProfileMenu, setShowProfileMenu] = useState(false);
  const [showChatMenu, setShowChatMenu] = useState(false);
  const [minimized, setMinimized] = useState(false);
  const winRef = useRef<HTMLDivElement>(null);

  const handleMinimize = () => {
    const win = winRef.current;
    if (!win) return;

    if (!minimized) {
      // snapshot actual pixel position before WAAPI overrides the CSS transform
      const rect = win.getBoundingClientRect();
      win.style.transform = "none";
      win.style.top = `${rect.top}px`;
      win.style.left = `${rect.left}px`;
      win.style.width = `${rect.width}px`;
      win.style.height = `${rect.height}px`;

      // translate toward bottom-left corner while shrinking to 0
      const tx = -rect.left;
      const ty = window.innerHeight - rect.bottom;

      win.style.transformOrigin = "0% 100%";
      win.getAnimations().forEach((a) => a.cancel());
      const anim = win.animate(
        [
          { transform: "translate(0, 0) scale(1)" },
          { transform: `translate(${tx}px, ${ty}px) scale(0)` },
        ],
        { duration: 600, easing: "ease-in-out", fill: "forwards" }
      );
      anim.onfinish = () => setMinimized(true);
    } else {
      setMinimized(false);
      requestAnimationFrame(() => {
        const w = winRef.current;
        if (!w) return;
        const rect = w.getBoundingClientRect();
        const tx = -rect.left;
        const ty = window.innerHeight - rect.bottom;
        w.style.transformOrigin = "0% 100%";
        w.getAnimations().forEach((a) => a.cancel());
        w.animate(
          [
            { transform: `translate(${tx}px, ${ty}px) scale(0)` },
            { transform: "translate(0, 0) scale(1)" },
          ],
          { duration: 300, easing: "ease-out", fill: "forwards" }
        );
      });
    }
  };

  const handleFullscreen = () => {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  };

  const toggleProfileMenu = () => {
    setShowChatMenu(false);
    setShowProfileMenu((v) => !v);
  };

  const toggleChatMenu = () => {
    setShowProfileMenu(false);
    setShowChatMenu((v) => !v);
  };

  const handleClose = () => {
    const win = winRef.current;
    const finish = () => { if (onClose) onClose(); else navigate("/"); };
    if (!win) { finish(); return; }
    win.animate(
      [{ opacity: 1, transform: "scale(1)" }, { opacity: 0, transform: "scale(0.95)" }],
      { duration: 300, easing: "ease", fill: "forwards" }
    ).onfinish = finish;
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
            'Authorization': `Bearer ${localStorage.getItem("token")}`
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
          'Authorization': `Bearer ${localStorage.getItem("token")}`
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
    try {
      const response = await fetch(`${API_URL}/create-room`, {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${localStorage.getItem("token")}`
      },
      body: JSON.stringify({ room: roomName })
    });
    if (!response.ok) {
      const errorText = await response.text();
        alert('Failed to create room: ' + errorText);
        return;
      }
      alert("Room created successfully");
      window.location.reload();
    } catch (error) {
      alert('Error creating room: ' + error);
    }
  };
  return (
    <>
    {minimized && (
      <button className="window-taskbar-pill" onClick={handleMinimize}>
        {title}
      </button>
    )}
    <div
      ref={winRef}
      className="window"
      style={{ width, height, top, left, visibility: minimized ? "hidden" : undefined }}
    >
      <div className="window-header">
        <div className="window-header-top">
          <div className="window-title" title={title} aria-label={title}>
            <span className="window-title-icon" aria-hidden="true"></span>
            <span className="window-title-text">{title}</span>
          </div>
          <WindowControls
            onMinimize={handleMinimize}
            onFullscreen={handleFullscreen}
            onClose={handleClose}
          />
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
              {location.pathname !== "/" && location.pathname !== "/home" && (
                <button
                  id="leave-room"
                  className="menu-button"
                  title="Leave room"
                  aria-label="Leave room"
                  data-tooltip="Leave room"
                  onClick={handleClose}
                ></button>
              )}
              <div className="profile-menu-wrapper">
                <button
                  id="edit-profile"
                  className="menu-button"
                  title="Edit profile"
                  aria-label="Edit profile"
                  data-tooltip="Edit profile"
                  onClick={toggleProfileMenu}
                  aria-haspopup="true"
                  aria-expanded={showProfileMenu}
                ></button>
                {showProfileMenu && (
                  <div className="profile-menu" role="menu">
                    <button role="menuitem" onClick={() => { setShowUsernameForm(true); setShowProfileMenu(false); }}>Change username</button>
                    <button role="menuitem" onClick={() => { setShowPasswordForm(true); setShowProfileMenu(false); }}>Change password</button>
                  </div>
                )}
              </div>
              {/* Additional decorative/action icons */}
              <button id="icon-drive" className="menu-button" title="Drive" aria-label="Drive" data-tooltip="Drive"></button>
              <button id="icon-downloads" className="menu-button" title="Downloads" aria-label="Downloads" data-tooltip="Downloads"></button>
              <button id="icon-folder" className="menu-button" title="Folder" aria-label="Folder" data-tooltip="Folder"></button>
              <button id="icon-folder-alt" className="menu-button" title="Folder alt" aria-label="Folder alt" data-tooltip="Folder alt"></button>
              <button id="icon-music" className="menu-button" title="Music" aria-label="Music" data-tooltip="Music"></button>
              <button id="icon-speaker" className="menu-button" title="Speaker" aria-label="Speaker" data-tooltip="Speaker"></button>
              <div className="profile-menu-wrapper">
                <button
                  id="chat-options"
                  className="menu-button"
                  title="Chat options"
                  aria-label="Chat options"
                  data-tooltip="Chat options"
                  onClick={toggleChatMenu}
                  aria-haspopup="true"
                  aria-expanded={showChatMenu}
                ></button>
                {showChatMenu && (
                  <div className="profile-menu" role="menu">
                    <button role="menuitem" onClick={() => { onCreateRoom(); setShowChatMenu(false); }}>Create room</button>
                    <button role="menuitem" onClick={() => { onSignOut && onSignOut(); setShowChatMenu(false); }}>Sign out</button>
                  </div>
                )}
              </div>
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
    </>
  );
};

export default Window;
