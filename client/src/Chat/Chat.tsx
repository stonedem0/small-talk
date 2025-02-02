import React, { useEffect, useState, useRef } from "react";
import "./Chat.css";

interface ChatProps {
  username: string;
  roomName: string;
}

interface Message {
  username: string;
  message: string;
}

const Chat: React.FC<ChatProps> = ({ username, roomName }) => {
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState<string>("");
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!roomName) return; // Don't connect if no room is selected

    const fetchHistory = async () => {
      try {
        const response = await fetch(
          `http://localhost:8080/history?room=${roomName}`
        );
        if (!response.ok)
          throw new Error(`HTTP error! Status: ${response.status}`);
        const data: Message[] = await response.json();
        setMessages(data.reverse()); // Reverse so oldest messages appear first
      } catch (error) {
        console.error("❌ Error fetching chat history:", error);
      }
    };

    fetchHistory();

    // Close existing WebSocket before opening a new one
    if (ws.current) {
      ws.current.close();
    }

    console.log("🔌 Connecting WebSocket for room:", roomName);
    ws.current = new WebSocket(`ws://localhost:8080/ws?room=${roomName}`);

    ws.current.onopen = () => {
      console.log("✅ WebSocket connected");
      setIsConnected(true);
    };

    ws.current.onmessage = (event: MessageEvent) => {
      const msg: Message = JSON.parse(event.data);
      setMessages((prev) => [...prev, msg]); // Append new messages
    };

    ws.current.onerror = (error) => {
      console.error("❌ WebSocket Error:", error);
    };

    ws.current.onclose = () => {
      console.warn("⚠️ WebSocket closed");
      setIsConnected(false);
    };

    return () => {
      ws.current?.close();
      ws.current = null;
    };
  }, [roomName]); // Reconnect WebSocket when room changes

  const sendMessage = (e: React.FormEvent) => {
    e.preventDefault();

    if (
      !message.trim() ||
      !ws.current ||
      ws.current.readyState !== WebSocket.OPEN
    ) {
      console.warn(
        "⚠️ Cannot send message: WebSocket is not connected or message is empty."
      );
      return;
    }

    const msgObject = {
      username: username || "Anonymous",
      message: message.trim(),
    };

    ws.current.send(JSON.stringify(msgObject));
    setMessage("");
  };

  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  return (
    <div id="chat-container">
      {roomName ? (
        <>
          <div className="chat-header">
            <span className="chat-name">{roomName}</span>
          </div>
          <div id="messages">
            {messages.map((msg, index) => (
              <p key={index}>
                <strong>{msg.username}:</strong> {msg.message}
              </p>
            ))}
            <div ref={messagesEndRef} />
          </div>
          <form onSubmit={sendMessage} id="message-controls">
            <input
              type="text"
              placeholder="Message"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              disabled={!isConnected}
            />
            <input type="submit" value="Send" disabled={!isConnected} />
          </form>
        </>
      ) : (
        // 🔥 Make sure this is inside the main div so it does NOT cause extra renders
        <p className="chat-placeholder">Select a room to start chatting.</p>
      )}
    </div>
  );
};

export default Chat;
