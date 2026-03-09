import React from "react";
import "./CancelButton.css";

interface CancelButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
}

const CancelButton = ({ children, ...props }: CancelButtonProps) => {
  return (
    <button className="cancel-xp-btn" {...props}>
      {children}
    </button>
  );
};

export default CancelButton;
