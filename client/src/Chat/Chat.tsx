import React, { useEffect, useState, useRef } from "react";
import "./Chat.css";

interface ChatProps {
  username: string;
}

interface Message {
  username: string;
  message: string;
}

const Chat: React.FC<ChatProps> = ({ username }) => {
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState<string>("");
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    const fetchHistory = async () => {
      try {
        const response = await fetch("http://localhost:8080/history");
        if (!response.ok)
          throw new Error(`HTTP error! Status: ${response.status}`);
        const data: Message[] = await response.json();
        console.log("📜 Loaded chat history:", data);
        setMessages(data.reverse()); // ✅ Reverse order so oldest messages appear first
      } catch (error) {
        console.error("❌ Error fetching chat history:", error);
      }
    };

    fetchHistory();

    if (!ws.current) {
      console.log("🔌 Creating WebSocket connection...");
      ws.current = new WebSocket("ws://localhost:8080/ws");

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
        console.warn("⚠️ WebSocket closed, attempting reconnect...");
        setIsConnected(false);
        ws.current = null; // Reset WebSocket reference
        setTimeout(() => reconnectWebSocket(), 3000);
      };
    }

    return () => {
      ws.current?.close();
      ws.current = null;
    };
  }, []);

  const reconnectWebSocket = () => {
    if (!ws.current || ws.current.readyState === WebSocket.CLOSED) {
      console.log("♻️ Reconnecting WebSocket...");
      ws.current = new WebSocket("ws://localhost:8080/ws");
    }
  };

  const sendMessage = (e: React.FormEvent) => {
    e.preventDefault();

    if (!message.trim()) {
      console.warn("⚠️ Cannot send an empty message");
      return;
    }

    if (!ws.current) {
      console.warn("⚠️ WebSocket instance is null. Cannot send message.");
      return;
    }

    if (ws.current.readyState === WebSocket.OPEN) {
      const msgObject = {
        username: username || "Anonymous",
        message: message.trim(),
      };

      console.log("📨 Sending:", msgObject);
      ws.current.send(JSON.stringify(msgObject));
      setMessage("");
    } else {
      console.warn("⚠️ WebSocket is not connected yet!");
    }
  };

  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  console.log(
    "📦 Chat messages:",
    messages.map((msg) => msg.message)
  );

  return (
    <div id="chat-container">
      <div className="chat-header">
        <span className="chat-name">Chat</span>
      </div>
      <div id="messages">
        {messages.map((msg, index) => (
          <p key={index}>
            <strong>{msg.username}:</strong> {msg.message}
          </p>
        ))}
        <div ref={messagesEndRef} /> {/* Scroll into view */}
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
    </div>
  );
};

export default Chat;
