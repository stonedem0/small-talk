// components/Loader.tsx
import "./Loader.css";

const Loader = ({ text = "Loading chat..." }: { text?: string }) => (
  <div className="loader-wrapper">
    <div className="loader-spinner">⌛</div>
    <div className="loader-text">{text}</div>
  </div>
);

export default Loader;
