import React from "react";
import "./PrimaryButton.css";

type ButtonSize = "xs" | "sm" | "md" | "lg";

interface PrimaryButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
  size?: ButtonSize;
}

const PrimaryButton = ({ children, size = "md", ...props }: PrimaryButtonProps) => {
  return (
    <button className={`primary-xp-btn primary-xp-btn--${size}`} {...props}>
      {children}
    </button>
  );
};

export default PrimaryButton;
