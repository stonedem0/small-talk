import WindowControls from "./WindowControls";
import "./Window.css";
import PrimaryButton from "./PrimaryButton";

type WindowProps = {
    title: string;
    children: React.ReactNode;
    width?: number;
    height?: number;
    username?: string | null;
    onSignOut?: () => void;
    // ... other props like draggable, resizable etc
  };

  
  const Window = ({
    title,
    children,
    width = 400,
    height = 300,
    username,
    onSignOut,
  }: WindowProps) => {
    return (
      <div className="window" style={{ width, height }}>
        <div className="window-header">
          <span>{title}</span>
          <WindowControls />
        </div>
  
        <div className="window-content">
          {/* Only show if both username and onSignOut are provided */}
          {username && onSignOut && (
            <div className="user-header">
              <span className="username">oh hai, {username}!</span>
              <PrimaryButton onClick={onSignOut}>Sign out</PrimaryButton>
            </div>
          )}
          {children}
        </div>
      </div>
    );
  };
  

  export default Window;