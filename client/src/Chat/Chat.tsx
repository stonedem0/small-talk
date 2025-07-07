import React, { useState, useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import "./Chat.css";
import { API_URL, WS_URL } from "../config";
import { format } from 'date-fns';

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
          ws.current?.send(
            JSON.stringify({
              type: "join",
              room: roomName,
              username: username,
              message: "joined the room"
            })
          );
        };

        ws.current.onmessage = (event) => {
          const newMessage: Message = JSON.parse(event.data);
          // TODO: probably need to handle this better:
          // if (newMessage.type == "system") {
          //   console.log("New message: ")
          //   console.log(newMessage)
          //   return;
          // }
          setMessages((prev) => [...prev, newMessage]);
        };

        ws.current.onclose = () => setIsConnected(false);
      } catch (error) {
        console.error("Failed to set up chat:", error);
      }
    };
    setupChat();
    return () => {
      if (ws.current && ws.current.readyState === WebSocket.OPEN) {
        ws.current.close();
      } else {
        ws.current?.close();
      }
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

  return (
    <div id="chat-container">
      <div className="chat-room">
        <div id="messages">
          {isLoadingMessages ? (
            <MessageSkeleton />
          ) : (
            messages.map((msg, index) => {
              // console.log(msg)
              let timeStr = '';
              if ((msg as any).timestamp) {
                try {
                  timeStr = format(new Date((msg as any).timestamp), 'HH:mm:ss');
                } catch {}
              }
              if ((msg as any).type === "system") {
                return (
                  <p key={index} style={{ color: "#888", fontStyle: "italic" }}>
                    {timeStr && <span>[{timeStr}] </span>}
                    {msg.username} {msg.message}
                  </p>
                );
              }
              return (
                <p key={index}>
                  {timeStr && <span>[{timeStr}] </span>}
                  <strong>{msg.username}:</strong> {msg.message}
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
            <input type="submit" value="send" id="send-message" />
          </form>
        </div>
      </div>
    </div>
  );
};

export default Chat;
