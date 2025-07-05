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
  tabs?: string[]; // ✅ NEW
  activeTab?: string; // ✅ Optional: control current tab externally
  onTabClick?: (tab: string) => void; // ✅ Optional: allow tab switch
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
  tabs,
  activeTab,
  onTabClick,
}: WindowProps) => {
  return (
    <div className="window" style={{ width, height, top, left }}>
      <div className="window-header">
        <span>{title}</span>
        <WindowControls />
      </div>

      {/* ✅ Optional Tabs Bar */}
      {tabs && tabs.length > 0 && (
        <div className="chat-tabs">
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
      )}

      <div className="window-content">
        {username && onSignOut && (
          <div className="window-menu-container">
            <div className="window-menu">
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
