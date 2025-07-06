import { useNavigate } from "react-router-dom";
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

  const handleClose = () => {
    if (onClose) {
      onClose();
    } else {
      navigate("/"); // ✅ fallback navigation
    }
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
                onClick={() => {
                  const newUsername = prompt("Enter your new username:");
                  if (newUsername) {
                    localStorage.setItem("username", newUsername);
                    window.location.reload();
                  }
                }}
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
        {children}
      </div>
    </div>
  );
};

export default Window;
