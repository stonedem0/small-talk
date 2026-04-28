import React from "react";
import "./DropdownMenu.css";

interface Props {
  header?: string;
  position?: "absolute" | "fixed";
  style?: React.CSSProperties;
  onClick?: (e: React.MouseEvent) => void;
  children: React.ReactNode;
}

const DropdownMenu = ({ header, position = "absolute", style, onClick, children }: Props) => (
  <div
    className="dropdown-menu"
    role="menu"
    style={{ position, ...style }}
    onClick={onClick}
  >
    {header && <div className="dropdown-menu-header">{header}</div>}
    {children}
  </div>
);

export default DropdownMenu;
