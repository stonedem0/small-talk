import React from "react";
import "./ModalWindow.css";

interface ModalWindowProps {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}

const ModalWindow = ({ title, onClose, children }: ModalWindowProps) => {
  return (
    <div className="modal-overlay" onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="modal-window">
        <div className="modal-window-title">{title}</div>
        <div className="modal-window-body">{children}</div>
      </div>
    </div>
  );
};

export default ModalWindow;
