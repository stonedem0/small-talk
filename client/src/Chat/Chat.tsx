import React, { useState, useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import "./Chat.css";
import { API_URL, WS_URL } from "../config";
import { format } from 'date-fns';
import PrimaryButton from "../components/PrimaryButton";

interface ChatProps {
  username: string;
}

interface Message {
  type: string;
  username: string;
  message: string;
  timestamp: string;
}

const Chat = ({ username }: ChatProps) => {
  const { roomName } = useParams<{ roomName: string }>();
  const navigate = useNavigate();

  const [validRooms, setValidRooms] = useState<string[]>([]);
  const [isValidRoom, setIsValidRoom] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(true);
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState("");
  const [isConnected, setIsConnected] = useState(false);
  const [onlineUsers, setOnlineUsers] = useState<string[]>([]);

  const ws = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const fetchRooms = async () => {
      try {
        const response = await fetch(`${API_URL}/rooms`, {
          headers: {
            "Authorization": `Bearer ${localStorage.getItem("token")}`
          }
        });
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
        const response = await fetch(`${API_URL}/history?room=${roomName}`, {
          headers: {
            "Authorization": `Bearer ${localStorage.getItem("token")}`
          }
        });
        if (!response.ok) throw new Error("Failed to fetch history");
        const data: Message[] = await response.json();
        if ( data && data.length > 0) {
          setMessages(data.reverse());
        } else {
          setMessages([]);
        }
        console.log('🔧 Creating WebSocket connection for room:', roomName, 'username:', username);
        ws.current = new WebSocket(`${WS_URL}/ws?room=${roomName}&username=${encodeURIComponent(username)}`);
        
        ws.current.onopen = () => {
          console.log("🔧 WebSocket connected successfully");
          setIsConnected(true);
          setIsLoadingMessages(false);
        };
        
        // Make WebSocket accessible globally for username updates
        (window as any).currentWebSocket = ws.current;
        console.log('🔧 WebSocket made globally accessible:', ws.current);

        ws.current.onmessage = (event) => {
          console.log('🔧 Received WebSocket message:', event.data);
          const newMessage: Message = JSON.parse(event.data);
          console.log('🔧 Parsed message:', newMessage);
        
          setMessages((prev) => [...prev, newMessage]);
        };

        ws.current.onclose = () => {
          console.log('🔧 WebSocket connection closed');
          setIsConnected(false);
        };
      } catch (error) {
        console.error("Failed to set up chat:", error);
      }
    };
    setupChat();
    return () => {
      console.log('🔧 Cleaning up WebSocket connection');
      if (ws.current && ws.current.readyState === WebSocket.OPEN) {
        console.log('🔧 Closing open WebSocket connection');
        ws.current.close();
      } else {
        console.log('🔧 WebSocket was not open, closing anyway');
        ws.current?.close();
      }
      // Clean up global WebSocket reference
      console.log('🔧 Removing global WebSocket reference');
      delete (window as any).currentWebSocket;
    };
  }, [isValidRoom, roomName, username]);

  useEffect(() => {
    if (!isValidRoom) return;

    let interval: number;
    const fetchOnlineUsers = async () => {
      try {
        const response = await fetch(`${API_URL}/room-usernames`);
        if (!response.ok) throw new Error("Failed to fetch online users");
        const data: Record<string, string[]> = await response.json();
        setOnlineUsers(data[roomName!] || []);
      } catch (error) {
        console.error("Failed to fetch online users:", error);
      }
    };
    fetchOnlineUsers();
    interval = window.setInterval(fetchOnlineUsers, 1000); // refresh every 1s
    return () => clearInterval(interval);
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
  }

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

  // Retro pixel art palette colors - pinks, greens, purples
  const getUserColor = (username: string) => {
    const colors = [
      "#ff6ec7", // Bright pink
      "#00ff7f", // Spring green
      "#9d4edd", // Deep purple
      "#ff1493", // Deep pink
      "#39ff14", // Neon green
      "#8a2be2", // Blue violet
      "#ff69b4", // Hot pink
      "#00fa9a", // Medium spring green
      "#dda0dd", // Plum
      "#ff91a4", // Light pink
      "#32cd32", // Lime green
      "#ba55d3", // Medium orchid
    ];
    
    // Generate consistent color based on username
    let hash = 0;
    for (let i = 0; i < username.length; i++) {
      hash = username.charCodeAt(i) + ((hash << 5) - hash);
    }
    return colors[Math.abs(hash) % colors.length];
  };

  return (
    <div id="chat-container">
      <div className="chat-room" style={{ display: "flex", flexDirection: "row" }}>
        <div style={{ flex: 1, display: "flex", flexDirection: "column" }}>
          <div id="messages">
            {isLoadingMessages ? (
              <MessageSkeleton />
            ) : (
              messages.map((msg, index) => {
                let timeStr = '';
                if ((msg as any).timestamp) {
                  try {
                    timeStr = format(new Date((msg as any).timestamp), 'HH:mm:ss');
                  } catch {}
                }
                if ((msg as any).type === "system") {
                  return (
                    <p key={index} style={{ background: "linear-gradient(90deg, rgba(139, 92, 246, 0.4), rgba(236, 72, 153, 0.4))", WebkitBackgroundClip: "text", WebkitTextFillColor: "transparent", backgroundClip: "text", fontStyle: "italic", opacity: 0.7 }}>
                      {timeStr && <span>[{timeStr}] </span>}
                      {msg.username} {msg.message}
                    </p>
                  );
                }
                return (
                  <p key={index}>
                    {timeStr && <span style={{ color: "#c084fc" }}>[{timeStr}] </span>}
                    <strong style={{ color: "#ff69b4" }}>{msg.username}:</strong> 
                    <span style={{ color: "#8b5cf6" }}> {msg.message}</span>
                  </p>
                );
              })
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
              <PrimaryButton type="submit" id="send-message">send</PrimaryButton>
            </form>
          </div>
        </div>
        <div className="online-users-sidebar">
          <h4>Online</h4>
            <ul>
              {onlineUsers.map((user) => (
                <li 
                  key={user} 
                  style={{ 
                    color: getUserColor(user),
                    fontWeight: "bold",
                    marginBottom: "4px"
                  }}
                >
                  ● {user}
                </li>
              ))}
            </ul>
        </div>
      </div>
    </div>
  );
};

export default Chat;
