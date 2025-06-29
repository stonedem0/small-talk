import React from "react";
import "./WindowControls.css";

interface WindowControlsProps {
  onMinimize?: () => void;
  onFullscreen?: () => void;
  onClose?: () => void;
}

const defaultMinimize = () => console.log("Minimize clicked");
const defaultFullscreen = () => console.log("Fullscreen clicked");
const defaultClose = () => console.log("Close clicked");

const WindowControls: React.FC<WindowControlsProps> = ({
  onMinimize = defaultMinimize,
  onFullscreen = defaultFullscreen,
  onClose = defaultClose,
}) => {
  return (
      <div className="window-controls">
      <button className="minimize" onClick={onMinimize}></button>
      <button className="fullscreen" onClick={onFullscreen}></button>
      <button className="close" onClick={onClose}></button>
    </div>
  );
};

export default WindowControls; 