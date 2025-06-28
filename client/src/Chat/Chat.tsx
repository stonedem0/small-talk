import React, { useState, useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import "./Chat.css";
import { API_URL, WS_URL } from "../config";
import WindowControls from "../components/WindowControls";

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
  const [isValidRoom, setIsValidRoom] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState<string>("");
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const ws = useRef<WebSocket | null>(null);

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
          console.warn("🚫 Invalid room:", roomName, validRooms);
          navigate("/");
        }
      } catch (error) {
        console.error("❌ Error fetching rooms:", error);
      } finally {
        setIsLoading(false);
      }
    };

    fetchRooms();
  }, [roomName, navigate]);

  useEffect(() => {
    if (!isValidRoom) return;

    const fetchHistory = async () => {
      try {
        const response = await fetch(`${API_URL}/history?room=${roomName}`);
        if (!response.ok)
          throw new Error(`HTTP error! Status: ${response.status}`);
        const data: Message[] = await response.json();
        setMessages(data.reverse()); // Do not reverse the order
      } catch (error) {
        console.error("Failed to fetch chat history:", error);
      }
    };

    fetchHistory();

    ws.current = new WebSocket(`${WS_URL}/ws?room=${roomName}`);
    ws.current.onopen = () => {
      setIsConnected(true);
    };
    ws.current.onmessage = (event) => {
      const newMessage: Message = JSON.parse(event.data);
      setMessages((prevMessages) => [...prevMessages, newMessage]); // Add new messages to the end
    };
    ws.current.onclose = () => {
      setIsConnected(false);
    };

    return () => {
      if (ws.current) {
        ws.current.close();
      }
    };
  }, [roomName, isValidRoom]);

  const sendMessage = (event: React.FormEvent) => {
    event.preventDefault();
    if (ws.current && message.trim()) {
      ws.current.send(JSON.stringify({ username, message }));
      setMessage("");
    }
  };

  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleMinimize = () => {
    console.log("Chat minimize clicked");
  };

  const handleFullscreen = () => {
    const chatContainer = document.getElementById("chat-container");
    if (chatContainer) {
      if (!document.fullscreenElement) {
        chatContainer.requestFullscreen().catch((err) => {
          console.error(
            `Error attempting to enable full-screen mode: ${err.message} (${err.name})`
          );
        });
      } else {
        document.exitFullscreen();
      }
    }
  };

  const handleClose = () => {
    navigate("/");
  };

  if (isLoading) {
    return <div className="spinner"></div>;
  }

  return (
    <div id="chat-container">
      {isValidRoom ? (
        <div className="chat-room">
          {/* <div className="chat-header">
            <span className="chat-name">{roomName}</span>
            <WindowControls
            onMinimize={handleMinimize}
            onFullscreen={handleFullscreen}
            onClose={handleClose}
          />
          </div> */}
          <div className="chat-menu">
            <button
              id="leave-room"
              className="menu-button"
              title="Leave room"
              onClick={() => {
                navigate("/");
              }}
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
          {/* </> */}
        </div>
      ) : (
        <div>Invalid room</div>
      )}
    </div>
  );
};

export default Chat;
