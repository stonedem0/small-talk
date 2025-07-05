import React, { useState, useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import "./Chat.css";
import { API_URL, WS_URL } from "../config";

import ChatSkeleton from "../components/ChatSkeleton";

interface ChatProps {
  username: string;
}

interface Message {
  username: string;
  message: string;
}

const Chat = ({ username }: ChatProps) => {
  const { roomName } = useParams<{ roomName: string }>();
  const navigate = useNavigate();

  const [validRooms, setValidRooms] = useState<string[]>([]);
  const [isValidRoom, setIsValidRoom] = useState<boolean>(false);
  const [isRoomReady, setIsRoomReady] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState<string>("");
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const ws = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  // Fetch valid rooms
  useEffect(() => {
    const fetchRooms = async () => {
      try {
        const response = await fetch(`${API_URL}/rooms`);
        if (!response.ok)
          throw new Error(`HTTP error! Status: ${response.status}`);
        const data: string[] = await response.json();
        setValidRooms(data);
        if (data.includes(roomName || "")) {
          setIsValidRoom(true);
        } else {
          console.warn("🚫 Invalid room:", roomName, data);
          navigate("/");
        }
      } catch (error) {
        console.error("❌ Error fetching rooms:", error);
      }
    };

    fetchRooms();
  }, [roomName, navigate]);

  // Connect to WebSocket and load history when room is valid
  useEffect(() => {
    if (!isValidRoom) return;

    const setupChat = async () => {
      try {
        // Fetch chat history
        const response = await fetch(`${API_URL}/history?room=${roomName}`);
        if (!response.ok)
          throw new Error(`HTTP error! Status: ${response.status}`);
        const data: Message[] = await response.json();
        setMessages(data.reverse());

        // Connect WebSocket
        ws.current = new WebSocket(`${WS_URL}/ws?room=${roomName}`);

        ws.current.onopen = () => {
          console.log("WebSocket connected");
          setIsConnected(true);
          setIsRoomReady(true); // ✅ Chat is fully ready
        };

        ws.current.onmessage = (event) => {
          const newMessage: Message = JSON.parse(event.data);
          setMessages((prev) => [...prev, newMessage]);
        };

        ws.current.onclose = () => {
          setIsConnected(false);
        };
      } catch (error) {
        console.error("Failed to set up chat:", error);
      }
    };

    setupChat();

    return () => {
      ws.current?.close();
    };
  }, [roomName, isValidRoom]);

  // Auto-scroll
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const sendMessage = (event: React.FormEvent) => {
    event.preventDefault();
    if (ws.current && message.trim()) {
      ws.current.send(JSON.stringify({ username, message }));
      setMessage("");
    }
  };

  const handleClose = () => navigate("/");
  const handleFullscreen = () => {
    const chatContainer = document.getElementById("chat-container");
    if (chatContainer) {
      if (!document.fullscreenElement) {
        chatContainer
          .requestFullscreen()
          .catch((err) => console.error("Fullscreen error:", err));
      } else {
        document.exitFullscreen();
      }
    }
  };
  const handleMinimize = () => console.log("Minimize clicked");

  return (
    <div id="chat-container">
      {isValidRoom && !isRoomReady ? (
        <ChatSkeleton />
      ) : (
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
            {messages.map((msg, index) => (
              <p key={index}>
                <strong>{msg.username}:</strong> {msg.message}
              </p>
            ))}
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
      )}
    </div>
  );
};

export default Chat;
