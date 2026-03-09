import React from "react";
import "./CancelButton.css";

type ButtonSize = "xs" | "sm" | "md" | "lg";

interface CancelButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
  size?: ButtonSize;
}

const CancelButton = ({ children, size = "md", ...props }: CancelButtonProps) => {
  return (
    <button className={`cancel-xp-btn cancel-xp-btn--${size}`} {...props}>
      {children}
    </button>
  );
};

export default CancelButton;
