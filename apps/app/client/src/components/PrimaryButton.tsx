import React from "react";
import "./PrimaryButton.css";

interface PrimaryButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
}

const PrimaryButton = ({ children, ...props }: PrimaryButtonProps) => {
  return (
    <button className="primary-xp-btn" {...props}>
      {children}
    </button>
  );
};

export default PrimaryButton; 