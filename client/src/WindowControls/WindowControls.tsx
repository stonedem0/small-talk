import React from "react";
import "./WindowControls.css";

interface WindowControlsProps {
  onMinimize?: () => void;
  onFullscreen?: () => void;
  onClose?: () => void;
}

const WindowControls: React.FC<WindowControlsProps> = ({
  onMinimize,
  onFullscreen,
  onClose,
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