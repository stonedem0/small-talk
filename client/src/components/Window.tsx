import WindowControls from "./WindowControls";
import "./Window.css";
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
  };

  
  const Window = ({
    title,
    children,
    width = 400,
    height = 300,
    top = '30%',
    left = '50%',
    username,
    onSignOut,
  }: WindowProps) => {
    return (
      <div className="window" style={{ width, height, top, left }}>
        <div className="window-header">
          <span>{title}</span>
          <WindowControls />
        </div>
        <div className="window-content">
          {username && onSignOut && (
            <div className="window-menu-container">
              <div className="window-menu">
                {/* <div className="window-menu-item">
                  <span>File</span>
                </div>
                <div className="window-menu-item">
                  <span>Edit</span>
                </div> */}
            <div className="sign-out">
              <span className="username">oh hai, <strong>{username}</strong>!</span>
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