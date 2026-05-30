import React, { useState, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useParams, useNavigate } from "react-router-dom";
import "./Chat.css";
import { authFetch } from "../utils/authFetch";
import { useSmallTalk, apiUrlRef } from "../context";
import { format } from 'date-fns';
import PrimaryButton from "../components/PrimaryButton";
import DropdownMenu from "../components/DropdownMenu";

const DIR_URL = (import.meta as any).env?.VITE_DIRECTORY_URL || "http://localhost:8081";
// Simple sanitizer that whitelists a small set of tags/attrs
const sanitizeHtml = (dirty: string): string => {
  const allowedTags = new Set(["STRONG","EM","U","DEL","CODE","A"]);
  const allowedAttrs = new Set(["href","target","rel","class"]);
  const container = document.createElement("div");
  container.innerHTML = dirty;

  const walk = (node: Node) => {
    if (node.nodeType === Node.ELEMENT_NODE) {
      const el = node as HTMLElement;
      const tag = el.tagName;
      if (!allowedTags.has(tag)) {
        const text = document.createTextNode(el.textContent || "");
        el.replaceWith(text);
        return; 
      }
      for (const attr of Array.from(el.attributes)) {
        if (!allowedAttrs.has(attr.name)) {
          el.removeAttribute(attr.name);
          continue;
        }
        if (el.tagName === "A" && attr.name === "href") {
          const value = attr.value.trim();
          if (!/^https?:\/\//i.test(value)) {
            el.removeAttribute("href");
          }
        }
      }
      if (el.tagName === "A") {
        el.setAttribute("rel", "noopener noreferrer");
        if (el.getAttribute("target") !== "_blank") {
          el.setAttribute("target", "_blank");
        }
        el.classList.add("msg-link");
      }
      if (el.tagName === "CODE") {
        el.classList.add("msg-code");
      }
    }
    for (const child of Array.from(node.childNodes)) {
      walk(child);
    }
  };
  walk(container);
  return container.innerHTML;
};

interface ChatProps {
  username: string;
  roomNameOverride?: string;
}

interface Message {
  type: string;
  username: string;
  message: string;
  timestamp: string;
}

const Chat = ({ username, roomNameOverride }: ChatProps) => {
  const { token, wsRef } = useSmallTalk();
  const params = useParams<{ roomName: string }>();
  const roomName = roomNameOverride ?? params.roomName;
  const navigate = useNavigate();

  const [isValidRoom, setIsValidRoom] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(true);
  const [messages, setMessages] = useState<Message[]>([]);
  const [message, setMessage] = useState("");
  const [onlineUsers, setOnlineUsers] = useState<string[]>([]);
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [showLinkForm, setShowLinkForm] = useState(false);
  const [linkUrl, setLinkUrl] = useState<string>("https://");
  const [linkText, setLinkText] = useState<string>("");
  const [typingUsers, setTypingUsers] = useState<{ [user: string]: ReturnType<typeof setTimeout> }>({});
  const [friends, setFriends] = useState<Set<string>>(new Set());
  const [sentRequests, setSentRequests] = useState<Set<string>>(new Set());
  const [userPopup, setUserPopup] = useState<{ username: string; x: number; y: number } | null>(null);
  const [confirmFriend, setConfirmFriend] = useState<string | null>(null);
  const [friendToast, setFriendToast] = useState<string | null>(null);
  const [userStatuses, setUserStatuses] = useState<Record<string, string>>({});

  const ws = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const EMOJIS = [
    "😀","😃","😄","😁","😆","😅","😂","🙂","😉","😊",
    "😍","😘","😜","🤪","😎","🤓","😏","😬","🥳","🤯",
    "😇","🥰","🤗","🤔","🤤","😭","😤","😴","🤝","👍"
  ];

  useEffect(() => {
    const fetchRooms = async () => {
      if (roomName?.startsWith("dm:") && !roomNameOverride) {
        // DM rooms must be accessed via /dm/:username, not directly
        navigate("/");
        return;
      }
      if (roomNameOverride?.startsWith("dm:")) {
        setIsValidRoom(true);
        return;
      }
      try {
        const response = await authFetch(`${apiUrlRef.current}/rooms`);
        if (!response.ok) throw new Error("Failed to fetch rooms");
        const data: string[] = await response.json();
        if (data.includes(roomName || "")) {
          setIsValidRoom(true);
        } else {
          navigate("/");
        }
      } catch (error) {
        console.error("Failed to fetch rooms", error);
      }
    };

    fetchRooms();
  }, [roomName, navigate]);

  useEffect(() => {
    if (!isValidRoom) return;

    const setupChat = async () => {
      try {
        const response = await authFetch(`${apiUrlRef.current}/history?room=${roomName}`);
        if (response.status === 403 || response.status === 404) { navigate("/"); return; }
        if (!response.ok) throw new Error("Failed to fetch history");
        const data: Message[] = await response.json();
        if ( data && data.length > 0) {
          setMessages(data);
        } else {
          setMessages([]);
        }
        const joinRes = await authFetch(`${DIR_URL}/join?room=${encodeURIComponent(roomName!)}`);
        if (!joinRes.ok) throw new Error(`join failed: ${joinRes.status}`);
        const { wss_url, app_id } = await joinRes.json();
        console.log("directory/join →", { wss_url, app_id });
        ws.current = new WebSocket(wss_url, [token ?? ""]);
        
        ws.current.onopen = () => {
          setIsLoadingMessages(false);
        };
        ws.current.onerror = (e) => {
          console.error("WebSocket error", e);
        };
        
        wsRef.current = ws.current;

        ws.current.onmessage = (event) => {
          const newMessage: Message = JSON.parse(event.data);

          if (newMessage.type === "typing") {
            if (newMessage.username !== username) {
              setTypingUsers((prev) => {
                if (prev[newMessage.username]) clearTimeout(prev[newMessage.username]);
                const timer = setTimeout(() => {
                  setTypingUsers((p) => { const n = { ...p }; delete n[newMessage.username]; return n; });
                }, 3000);
                return { ...prev, [newMessage.username]: timer };
              });
            }
            return;
          }
          if (newMessage.type === "stop_typing") {
            setTypingUsers((prev) => {
              if (prev[newMessage.username]) clearTimeout(prev[newMessage.username]);
              const n = { ...prev }; delete n[newMessage.username]; return n;
            });
            return;
          }
          if (newMessage.type === "status_update") {
            setUserStatuses((prev) => ({ ...prev, [newMessage.username]: newMessage.message }));
            return;
          }

          setMessages((prev) => [...prev, newMessage]);
        };

        ws.current.onclose = () => {};
      } catch (error) {
        console.error("Failed to setup chat", error);
      }
    };
    setupChat();
    return () => {
      if (ws.current && ws.current.readyState === WebSocket.OPEN) {
        ws.current.close();
      } else {
        ws.current?.close();
      }
      // Clean up global WebSocket reference
      delete (window as any).currentWebSocket;
    };
  }, [isValidRoom, roomName, username]);

  useEffect(() => {
    if (!isValidRoom) return;

    let interval: number;
    const fetchOnlineUsers = async () => {
      try {
        const response = await authFetch(`${apiUrlRef.current}/room-usernames`);
        if (!response.ok) throw new Error("Failed to fetch online users");
        const data: Record<string, string[]> = await response.json();
        const users = data[roomName!] || [];
        setOnlineUsers(users);
        // Fetch statuses for all online users in one request
        if (users.length > 0) {
          const sr = await authFetch(`${apiUrlRef.current}/statuses?usernames=${users.join(",")}`);
          if (sr.ok) {
            const statuses: Record<string, string> = await sr.json();
            setUserStatuses(statuses);
          }
        }
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

  useEffect(() => {
    authFetch(`${apiUrlRef.current}/friends`)
      .then(r => r.ok ? r.json() : [])
      .then((list: string[]) => setFriends(new Set(list)))
      .catch(() => {});
    authFetch(`${apiUrlRef.current}/friends/sent`)
      .then(r => r.ok ? r.json() : [])
      .then((list: string[]) => setSentRequests(new Set(list)))
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (!userPopup) return;
    const close = () => { setUserPopup(null); setConfirmFriend(null); };
    window.addEventListener("click", close);
    return () => window.removeEventListener("click", close);
  }, [userPopup]);

  const handleUsernameClick = (e: React.MouseEvent, target: string) => {
    if (target === username) return;
    e.stopPropagation();
    setConfirmFriend(null);
    setUserPopup({ username: target, x: e.clientX, y: e.clientY });
  };

  const sendFriendRequest = async (target: string) => {
    const res = await authFetch(`${apiUrlRef.current}/friends/request`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target })
    });
    setConfirmFriend(null);
    setUserPopup(null);
    if (res.status === 409) {
      const data = await res.json().catch(() => ({}));
      const msg = data.error === "already friends"
        ? `You're already friends with ${target}`
        : `Request already sent to ${target}`;
      setSentRequests(prev => new Set(prev).add(target));
      setFriendToast(msg);
    } else if (res.ok) {
      setSentRequests(prev => new Set(prev).add(target));
      setFriendToast(`Friend request sent to ${target}!`);
    }
    setTimeout(() => setFriendToast(null), 3000);
  };

  const openDM = (target: string) => {
    navigate(`/dm/${target}`);
  };

  const sendTyping = () => {
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) return;
    ws.current.send(JSON.stringify({ type: "typing", username }));
    if (typingTimeoutRef.current) clearTimeout(typingTimeoutRef.current);
    typingTimeoutRef.current = setTimeout(() => {
      ws.current?.send(JSON.stringify({ type: "stop_typing", username }));
    }, 2000);
  };

  const sendMessage = (e: React.FormEvent) => {
    e.preventDefault();
    if (ws.current && message.trim()) {
      if (typingTimeoutRef.current) clearTimeout(typingTimeoutRef.current);
      ws.current.send(JSON.stringify({ type: "stop_typing", username }));
      ws.current.send(JSON.stringify({ username, message }));
      setMessage("");
    }
  }

  

  const insertFormatting = (start: string, end: string) => {
    const input = inputRef.current;
    if (!input) return;

    const startPos = input.selectionStart || 0;
    const endPos = input.selectionEnd || 0;
    const selectedText = message.substring(startPos, endPos);
    
    const newText = 
      message.substring(0, startPos) + 
      start + selectedText + end + 
      message.substring(endPos);
    
    setMessage(newText);
    
    setTimeout(() => {
      input.focus();
      
      if (selectedText.length > 0) {
        const newCursorPos = startPos + start.length + selectedText.length + end.length;
        input.setSelectionRange(newCursorPos, newCursorPos);
      } else {
        const cursorPos = startPos + start.length;
        input.setSelectionRange(cursorPos, cursorPos);
      }
    }, 0);
  };

  const insertEmoji = (emoji: string) => {
    const input = inputRef.current;
    if (!input) {
      setMessage((prev) => prev + emoji);
      return;
    }
    const startPos = input.selectionStart ?? message.length;
    const endPos = input.selectionEnd ?? message.length;
    const newText = message.substring(0, startPos) + emoji + message.substring(endPos);
    setMessage(newText);
    setTimeout(() => {
      input.focus();
      const caret = startPos + emoji.length;
      input.setSelectionRange(caret, caret);
    }, 0);
  };

  const toggleLinkForm = () => {
    const input = inputRef.current;
    if (input) {
      const startPos = input.selectionStart ?? message.length;
      const endPos = input.selectionEnd ?? message.length;
      const selectedText = message.substring(startPos, endPos);
      setLinkText(selectedText);
    }
    setLinkUrl((prev) => (prev && prev !== "https://" ? prev : "https://"));
    setShowLinkForm((v) => !v);
  };

  const handleInsertLink = () => {
    if (!linkUrl || !/^https?:\/\//i.test(linkUrl.trim())) {
      // very light validation; require http/https
      setLinkUrl((u) => (u?.trim() ? u : "https://"));
      return;
    }
    const input = inputRef.current;
    const startPos = input?.selectionStart ?? message.length;
    const endPos = input?.selectionEnd ?? message.length;
    const label = (linkText && linkText.trim()) ? linkText.trim() : linkUrl.trim();
    const md = `[${label}](${linkUrl.trim()})`;
    const newText = message.substring(0, startPos) + md + message.substring(endPos);
    setMessage(newText);
    setShowLinkForm(false);
    setTimeout(() => {
      if (input) {
        input.focus();
        const caret = startPos + md.length;
        input.setSelectionRange(caret, caret);
      }
    }, 0);
  };

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
        <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
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
                    <p key={index} style={{ background: "linear-gradient(90deg, rgba(139, 92, 246, 0.5), rgba(236, 72, 153, 0.5))", WebkitBackgroundClip: "text", WebkitTextFillColor: "transparent", backgroundClip: "text", fontStyle: "italic", opacity: 0.8 }}>
                      {timeStr && <span>[{timeStr}] </span>}
                      {msg.username} {msg.message}
                    </p>
                  );
                }
                // Format message text with basic markdown
                const formatMessage = (text: string) => {
                  // Replace **bold** with <strong>
                  text = text.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
                  // Replace *italic* with <em>
                  text = text.replace(/\*(.*?)\*/g, '<em>$1</em>');
                  // Replace _underline_ with <u>
                  text = text.replace(/_(.*?)_/g, '<u>$1</u>');
                  // Replace ~~strikethrough~~ with <del>
                  text = text.replace(/~~(.*?)~~/g, '<del>$1</del>');
                  // Replace `code` with <code>
                  text = text.replace(/`(.*?)`/g, '<code class="msg-code">$1</code>');
                  // Replace [text](url) with link
                  text = text.replace(/\[(.*?)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer" class="msg-link">$1</a>');
                  // Auto-link plain URLs (simple)
                  text = text.replace(/(^|\s)(https?:\/\/[^\s<]+[^<.,)\s])/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer" class="msg-link">$2</a>');
                  return sanitizeHtml(text);
                };

                return (
                  <p key={index}>
                    {timeStr && <span style={{ color: "#c084fc" }}>[{timeStr}] </span>}
                    <strong
                      style={{ color: "#ff69b4", cursor: msg.username !== username ? "pointer" : "default" }}
                      onClick={(e) => handleUsernameClick(e, msg.username)}
                    >
                      {msg.username}{friends.has(msg.username) && <span title="friend" style={{ color: "#ff69b4", fontSize: "0.8em" }}> ♥</span>}:
                    </strong>
                    <span
                      className="msg-text"
                      dangerouslySetInnerHTML={{ __html: " " + formatMessage(msg.message) }}
                    />
                  </p>
                );
              })
            )}
            {Object.keys(typingUsers).length > 0 && (
              <p className="typing-indicator">
                {Object.keys(typingUsers).join(", ")} {Object.keys(typingUsers).length === 1 ? "is" : "are"} typing...
              </p>
            )}
            <div ref={messagesEndRef} />
          </div>

          <div id="message-controls" className="message-controls-container">
            <div className="formatting-toolbar">
              <button
                type="button"
                className="formatting-button bold"
                data-tooltip="Bold (**text**)"
                onClick={() => insertFormatting("**", "**")}
              >
                B
              </button>
              <button
                type="button"
                className="formatting-button italic"
                data-tooltip="Italic (*text*)"
                onClick={() => insertFormatting("*", "*")}
              >
                I
              </button>
              <button
                type="button"
                className="formatting-button underline"
                data-tooltip="Underline (_text_)"
                onClick={() => insertFormatting("_", "_")}
              >
                U
              </button>
              <button
                type="button"
                className="formatting-button code"
                data-tooltip="Code (`text`)"
                onClick={() => insertFormatting("`", "`")}
              >
                &lt;/&gt;
              </button>
              <button
                type="button"
                className="formatting-button strikethrough"
                data-tooltip="Strikethrough (~~text~~)"
                onClick={() => insertFormatting("~~", "~~")}
              >
                S
              </button>
              <button
                type="button"
                className="formatting-button link"
                data-tooltip="Insert link"
                onClick={toggleLinkForm}
              >
                🔗
              </button>
              <button
                type="button"
                className="formatting-button emoji-toggle"
                data-tooltip="Emoji"
                onClick={() => setShowEmojiPicker((v) => !v)}
                aria-haspopup="true"
                aria-expanded={showEmojiPicker}
              >
                😊
              </button>
            </div>
            {showLinkForm && (
              <div className="link-form" role="group" aria-label="Insert link">
                <input
                  type="text"
                  className="link-input"
                  placeholder="https://example.com"
                  value={linkUrl}
                  onChange={(e) => setLinkUrl(e.target.value)}
                />
                <input
                  type="text"
                  className="link-text-input"
                  placeholder="Link text (optional)"
                  value={linkText}
                  onChange={(e) => setLinkText(e.target.value)}
                />
                <button type="button" className="link-insert" onClick={handleInsertLink}>Insert</button>
                <button type="button" className="link-cancel" onClick={() => setShowLinkForm(false)}>Cancel</button>
              </div>
            )}
            {showEmojiPicker && (
              <div className="emoji-picker" role="listbox">
                {EMOJIS.map((e) => (
                  <button
                    type="button"
                    key={e}
                    className="emoji-item"
                    onClick={() => { insertEmoji(e); setShowEmojiPicker(false); }}
                    aria-label={`emoji ${e}`}
                  >
                    {e}
                  </button>
                ))}
              </div>
            )}
            <div className="message-input-container">
            <form onSubmit={sendMessage} id="submit" className="message-input-form">
              <input
                ref={(input) => { inputRef.current = input; }}
                placeholder="Type your message..."
                type="text"
                id="message"
                value={message}
                onChange={(e) => { setMessage(e.target.value); sendTyping(); }}
                className="message-input"
              />
              <div className="send-button-container">
                <PrimaryButton type="submit" id="send-message">send</PrimaryButton>
              </div>
            </form>
            </div>
          </div>
        </div>
        <div className="online-users-sidebar">
          <h4 style={{ marginTop: 0 }}>Online</h4>
            <ul>
              {onlineUsers
                .slice()
                .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()))
                .map((user) => (
                <li key={user} className="online-user-item">
                  <span
                    style={{ color: getUserColor(user), fontWeight: "bold", cursor: user !== username ? "pointer" : "default" }}
                    onClick={(e) => handleUsernameClick(e, user)}
                    title={user !== username ? `Click to message or add ${user}` : undefined}
                  >
                    ● {user}
                  </span>
                  {friends.has(user) && <span className="sidebar-friend-badge" title="friends">♥</span>}
                  {sentRequests.has(user) && <span className="sidebar-pending-badge" title="Request pending">~</span>}
                </li>
              ))}
            </ul>
        </div>
      </div>
      {friendToast && createPortal(
        <div className="friend-sent-toast">{friendToast}</div>,
        document.body
      )}
      {userPopup && createPortal(
        <DropdownMenu
          header={userPopup.username}
          position="fixed"
          style={{ top: userPopup.y + 8, left: userPopup.x + 8 }}
          onClick={(e) => e.stopPropagation()}
        >
          <button onClick={() => openDM(userPopup.username)}>message</button>
          {friends.has(userPopup.username) ? (
            <div className="dropdown-item-text" style={{ color: "#ff69b4" }}>♥ friends</div>
          ) : sentRequests.has(userPopup.username) ? (
            <div className="dropdown-item-text" style={{ color: "#a07ccc" }}>request pending</div>
          ) : confirmFriend === userPopup.username ? (
            <>
              <div className="dropdown-item-text">add {userPopup.username}?</div>
              <button style={{ color: "green" }} onClick={() => sendFriendRequest(userPopup.username)}>yes</button>
              <button style={{ color: "red" }} onClick={() => setConfirmFriend(null)}>no</button>
            </>
          ) : (
            <button onClick={() => setConfirmFriend(userPopup.username)}>add friend</button>
          )}
        </DropdownMenu>,
        document.body
      )}
    </div>
  );
};

export default Chat;
