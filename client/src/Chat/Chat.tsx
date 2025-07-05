import React, { useState, useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import "./Chat.css";
import { API_URL, WS_URL } from "../config";

interface ChatProps {
  username: string;
}

interface Message {
  username: string;
  message: string;
}

const Chat: React.FC<ChatProps> = ({ username }) => {
  const { roomName } = useParams<{ roomName: string }>();
  const navigate = useNavigate();

  const [validRooms, setValidRooms] = useState<string[]>([]);
  const [isValidRoom, setIsValidRoom] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(true);
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState("");
  const [isConnected, setIsConnected] = useState(false);

  const ws = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const fetchRooms = async () => {
      try {
        const response = await fetch(`${API_URL}/rooms`);
        if (!response.ok) throw new Error("Failed to fetch rooms");
        const data: string[] = await response.json();
        setValidRooms(data);
        if (data.includes(roomName || "")) {
          setIsValidRoom(true);
        } else {
          console.warn("🚫 Invalid room:", roomName);
          navigate("/");
        }
      } catch (error) {
        console.error("❌ Error fetching rooms:", error);
      }
    };

    fetchRooms();
  }, [roomName, navigate]);

  useEffect(() => {
    if (!isValidRoom) return;

    const setupChat = async () => {
      try {
        const response = await fetch(`${API_URL}/history?room=${roomName}`);
        if (!response.ok) throw new Error("Failed to fetch history");
        const data: Message[] = await response.json();
        setMessages(data.reverse());

        ws.current = new WebSocket(`${WS_URL}/ws?room=${roomName}`);

        ws.current.onopen = () => {
          console.log("WebSocket connected");
          setIsConnected(true);
          setIsLoadingMessages(false);
        };

        ws.current.onmessage = (event) => {
          const newMessage: Message = JSON.parse(event.data);
          setMessages((prev) => [...prev, newMessage]);
        };

        ws.current.onclose = () => setIsConnected(false);
      } catch (error) {
        console.error("Failed to set up chat:", error);
      }
    };

    setupChat();

    return () => {
      ws.current?.close();
    };
  }, [isValidRoom, roomName]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const sendMessage = (e: React.FormEvent) => {
    e.preventDefault();
    if (ws.current && message.trim()) {
      ws.current.send(JSON.stringify({ username, message }));
      setMessage("");
    }
  };

  const handleClose = () => navigate("/");

  const handleFullscreen = () => {
    const container = document.getElementById("chat-container");
    if (container) {
      if (!document.fullscreenElement) {
        container.requestFullscreen().catch(console.error);
      } else {
        document.exitFullscreen();
      }
    }
  };

  const handleMinimize = () => console.log("Minimize clicked");

  const MessageSkeleton = () => (
    <div className="message-skeleton-wrapper">
      {[...Array(8)].map((_, i) => (
        <div key={i} className="message-skeleton" />
      ))}
    </div>
  );

  return (
    <div id="chat-container">
      <div className="chat-room">
        <div className="chat-menu">
          <button
            id="leave-room"
            className="menu-button"
            title="Leave room"
            onClick={handleClose}
          ></button>
          <button
            id="change-username"
            className="menu-button"
            title="Change username"
            onClick={() => {
              const newUsername = prompt("Enter your new username:");
              if (newUsername) {
                localStorage.setItem("username", newUsername);
                window.location.reload();
              }
            }}
          ></button>
        </div>
        <div id="messages">
          {isLoadingMessages ? (
            <MessageSkeleton />
          ) : (
            messages.map((msg, index) => (
              <p key={index}>
                <strong>{msg.username}:</strong> {msg.message}
              </p>
            ))
          )}
          <div ref={messagesEndRef} />
        </div>

        <div id="message-controls">
          <form onSubmit={sendMessage} id="submit">
            <input
              placeholder="message"
              type="text"
              id="message"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
            />
            <input type="submit" value="send" id="send-message" />
          </form>
        </div>
      </div>
    </div>
  );
};

export default Chat;
