import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import "./Rooms.css";
import { API_URL } from "../config";
import { authFetch } from "../utils/authFetch";
import Chat from "../Chat/Chat";
import DMChat from "../Chat/DMChat";
import avatar from "../assets/avatar.png";

interface RoomsProps {
  unreadDMs?: { [from: string]: number };
  onDMOpen?: (from: string) => void;
}

type SelectedChat =
  | { type: "room"; name: string }
  | { type: "dm"; target: string }
  | null;

const Rooms = ({ unreadDMs = {}, onDMOpen }: RoomsProps) => {
  const [grouped, setGrouped] = useState<{ [category: string]: string[] }>({});
  const [collapsed, setCollapsed] = useState<{ [category: string]: boolean }>({});
  const [userCounts, setUserCounts] = useState<{ [room: string]: number }>({});
  const [username, setUsername] = useState<string | null>(null);
  const [dmMessages, setDmMessages] = useState<string[]>([]);
  const [friends, setFriends] = useState<string[]>([]);
  const [friendsCollapsed, setFriendsCollapsed] = useState(false);
  const [dmsCollapsed, setDmsCollapsed] = useState(false);
  const [myStatus, setMyStatus] = useState<string>("");
  const [selectedChat, setSelectedChat] = useState<SelectedChat>(() => {
    try { return JSON.parse(localStorage.getItem("rooms_selected_chat") ?? "null"); } catch { return null; }
  });
  const [contactsHidden, setContactsHidden] = useState(() => {
    const stored = localStorage.getItem("rooms_contacts_hidden");
    // default hidden whenever a chat is active
    if (stored !== null) return stored === "true";
    return localStorage.getItem("rooms_selected_chat") !== null;
  });

  const toggleCategory = (cat: string) =>
    setCollapsed((prev) => ({ ...prev, [cat]: !prev[cat] }));

  const fetchRooms = () => {
    authFetch(`${API_URL}/rooms-with-categories`)
      .then((response) => response.json())
      .then((data: { [cat: string]: string[] }) => {
        const sorted: { [cat: string]: string[] } = {};
        Object.keys(data).sort().forEach(cat => {
          sorted[cat] = data[cat].sort();
        });
        setGrouped(sorted);
        setCollapsed((prev) => {
          const next: { [cat: string]: boolean } = { ...prev };
          Object.keys(sorted).forEach(cat => {
            if (!(cat in next)) next[cat] = true;
          });
          return next;
        });
      })
      .catch((error) => console.error("rooms list fetch error", error));
  };

  useEffect(() => {
    const u = localStorage.getItem("username");
    if (u) {
      setUsername(u);
      authFetch(`${API_URL}/statuses?usernames=${u}`)
        .then((r) => r.json())
        .then((data: Record<string, string>) => setMyStatus(data[u] || ""))
        .catch(() => {});
    }
  }, []);

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) return;
    fetchRooms();

    authFetch(`${API_URL}/online-users`)
      .then((r) => r.json())
      .then((data) => setUserCounts(data))
      .catch(() => {});

    authFetch(`${API_URL}/dms`)
      .then((r) => r.json())
      .then((data: string[]) => setDmMessages(data || []))
      .catch(() => {});

    authFetch(`${API_URL}/friends`)
      .then((r) => r.json())
      .then((data: string[]) => setFriends(data || []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    localStorage.setItem("rooms_selected_chat", JSON.stringify(selectedChat));
  }, [selectedChat]);

  useEffect(() => {
    localStorage.setItem("rooms_contacts_hidden", String(contactsHidden));
  }, [contactsHidden]);

  const openDM = (partner: string) => {
    onDMOpen?.(partner);
    setSelectedChat({ type: "dm", target: partner });
    setContactsHidden(true);
  };

  return (
    <div id="rooms-container">

      {/* top bar */}
      <div className="rooms-topbar">
        <div className="rooms-topbar-avatar">
          <img src={avatar} alt="avatar" className="rooms-avatar-img" />
        </div>
        <div className="rooms-topbar-info">
          <span className="rooms-topbar-name">{username || "User"}</span>
          <span className="rooms-topbar-status">
            <span className="rooms-avatar-dot" />
            online
          </span>
          {myStatus && <span className="rooms-topbar-custom-status">{myStatus}</span>}
        </div>
        {selectedChat && (
          <>
            <button className="rooms-back-btn" onClick={() => { setSelectedChat(null); setContactsHidden(false); localStorage.removeItem("rooms_selected_chat"); localStorage.removeItem("rooms_contacts_hidden"); }} >
              ← back
            </button>
            <button className="rooms-toggle-btn" title={contactsHidden ? "show contacts" : "hide contacts"} onClick={() => setContactsHidden(v => !v)}>
              {contactsHidden ? "▶" : "◀"}
            </button>
          </>
        )}
      </div>

      <div className="rooms-body">

        {/* left col — contacts */}
        <div className={`rooms-contacts${contactsHidden ? " rooms-contacts--hidden" : ""}`}>

          {/* friends */}
          <div className="contact-group">
            <button className="contact-group-header" onClick={() => setFriendsCollapsed(v => !v)}>
              <span className={`contact-arrow ${friendsCollapsed ? "contact-arrow--closed" : ""}`} />
              friends ({friends.length})
            </button>
            {!friendsCollapsed && (
              <ul className="contact-list">
                {friends.length === 0 && (
                  <li className="contact-empty">no friends yet</li>
                )}
                {friends.sort().map((f) => (
                  <li key={f} className="contact-item">
                    <a href="#" onClick={(e) => { e.preventDefault(); openDM(f); }}>
                      <span className="contact-status-dot contact-status-dot--online" />
                      {f}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* dms */}
          {dmMessages.length > 0 && (
            <div className="contact-group">
              <button className="contact-group-header" onClick={() => setDmsCollapsed(v => !v)}>
                <span className={`contact-arrow ${dmsCollapsed ? "contact-arrow--closed" : ""}`} />
                messages ({dmMessages.length})
              </button>
              {!dmsCollapsed && (
                <ul className="contact-list">
                  {dmMessages.sort().map((partner) => (
                    <li key={partner} className="contact-item">
                      <a href="#" onClick={(e) => { e.preventDefault(); openDM(partner); }}>
                        <span className="contact-status-dot" />
                        {partner}
                        {unreadDMs[partner] ? <span className="contact-unread">{unreadDMs[partner]}</span> : null}
                      </a>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>

        {/* right col */}
        {selectedChat ? (
          <div className="rooms-chat-panel">
            {selectedChat.type === "room" && username && (
              <Chat username={username} roomNameOverride={selectedChat.name} />
            )}
            {selectedChat.type === "dm" && username && (
              <DMChat username={username} targetUsernameOverride={selectedChat.target} />
            )}
          </div>
        ) : (
          <div className="rooms-panel-wrapper">
            <div className="rooms-panel">
              <ul className="rooms-list">
                <li className="room-item room-item--rules">
                  <Link to="/rules">#rules</Link>
                </li>
                {Object.entries(grouped).map(([category, rooms]) => (
                  <li key={category} className="room-category">
                    <button className="room-category-label" onClick={() => toggleCategory(category)}>
                      <span className={`room-category-arrow ${collapsed[category] ? "room-category-arrow--closed" : ""}`} />
                      {category.toLowerCase()}
                    </button>
                    {!collapsed[category] && (
                      <ul className="room-category-list">
                        {rooms.map((room) => (
                          <li key={room} className="room-item">
                            <a href="#" onClick={(e) => { e.preventDefault(); setSelectedChat({ type: "room", name: room }); setContactsHidden(true); }}>
                              #{room}{userCounts[room] > 0 ? ` (${userCounts[room]})` : ""}
                            </a>
                          </li>
                        ))}
                      </ul>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        )}

      </div>
    </div>
  );
};

export default Rooms;
