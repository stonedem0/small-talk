import React from "react";
import "./ChatSkeleton.css";

const ChatSkeleton: React.FC = () => {
  return (
    <div className="chat-room skeleton">
      <div id="messages">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="message-skeleton" />
        ))}
      </div>
    </div>
  );
};

export default ChatSkeleton;
