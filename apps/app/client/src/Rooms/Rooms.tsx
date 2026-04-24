import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
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
  const [favorites, setFavorites] = useState<string[]>([]);
  const [editingStatus, setEditingStatus] = useState(false);
  const [statusDraft, setStatusDraft] = useState("");
  const [onlineSet, setOnlineSet] = useState<Set<string>>(new Set());
  const [roomSearch, setRoomSearch] = useState("");
  const [ping, setPing] = useState<number | null>(null);
  const [connected, setConnected] = useState(true);
  const [selectedChat, setSelectedChat] = useState<SelectedChat>(() => {
    try { return JSON.parse(localStorage.getItem("rooms_selected_chat") ?? "null"); } catch { return null; }
  });
  const [contactsHidden, setContactsHidden] = useState(() => {
    const stored = localStorage.getItem("rooms_contacts_hidden");
    // default hidden whenever a chat is active
    if (stored !== null) return stored === "true";
    return localStorage.getItem("rooms_selected_chat") !== null;
  });

  const location = useLocation();
  useEffect(() => {
    if ((location.state as any)?.goHome) {
      setSelectedChat(null);
      setContactsHidden(false);
    }
  }, [location.state]);

  useEffect(() => {
    const measure = async () => {
      const t0 = performance.now();
      try {
        await fetch(`${API_URL}/ping`);
        setPing(Math.round(performance.now() - t0));
        setConnected(true);
      } catch {
        setConnected(false);
      }
    };
    measure();
    const id = setInterval(measure, 5000);
    return () => clearInterval(id);
  }, []);

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
      .then((data: string[]) => {
        setFriends(data || []);
        // TEST: seed fake online status for first friend
        if (data && data.length > 0) setOnlineSet(new Set([data[0]]));
      })
      .catch(() => {});

    authFetch(`${API_URL}/favorites/list`)
      .then((r) => r.json())
      .then((data: string[]) => setFavorites(data || []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    const fetchOnline = () => {
      authFetch(`${API_URL}/room-usernames`)
        .then(r => r.json())
        .then((data: Record<string, string[]>) => {
          const all = new Set<string>();
          Object.values(data).forEach(users => users.forEach(u => all.add(u)));
          setOnlineSet(all);
        })
        .catch(() => {});
    };
    fetchOnline();
    const interval = window.setInterval(fetchOnline, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    localStorage.setItem("rooms_selected_chat", JSON.stringify(selectedChat));
  }, [selectedChat]);

  useEffect(() => {
    localStorage.setItem("rooms_contacts_hidden", String(contactsHidden));
  }, [contactsHidden]);

  const saveStatus = () => {
    setEditingStatus(false);
    const trimmed = statusDraft.trim().slice(0, 100);
    setMyStatus(trimmed);
    authFetch(`${API_URL}/status`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status: trimmed }),
    }).catch(() => {});
  };

  const toggleFavorite = (room: string) => {
    const isFav = favorites.includes(room);
    setFavorites(isFav ? favorites.filter(f => f !== room) : [...favorites, room]);
    authFetch(`${API_URL}/favorites`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ room }),
    }).catch(() => {});
  };

  const openDM = (partner: string) => {
    onDMOpen?.(partner);
    setSelectedChat({ type: "dm", target: partner });
    setContactsHidden(true);
  };

  return (
    <div id="rooms-container">
      <div className="rooms-body">

        {/* left col — home screen only */}
        {!selectedChat && <div className="rooms-left">
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
              {editingStatus ? (
                <input
                  className="rooms-status-input"
                  autoFocus
                  value={statusDraft}
                  maxLength={100}
                  placeholder="set a status…"
                  onChange={(e) => setStatusDraft(e.target.value)}
                  onBlur={saveStatus}
                  onKeyDown={(e) => { if (e.key === "Enter") saveStatus(); if (e.key === "Escape") setEditingStatus(false); }}
                />
              ) : (
                <span
                  className="rooms-topbar-custom-status rooms-topbar-custom-status--editable"
                  onClick={() => { setStatusDraft(myStatus); setEditingStatus(true); }}
                  title="click to edit status"
                >
                  {myStatus || "set a status…"}
                </span>
              )}
            </div>
          </div>

          <div className="rooms-contacts">
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
                        <span className={`contact-status-dot${onlineSet.has(f) ? " contact-status-dot--online" : ""}`} />
                        {f}
                      </a>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            {/* favorites */}
            {favorites.length > 0 && (
              <div className="contact-group">
                <button className="contact-group-header" onClick={() => {}}>
                  <span className="contact-arrow" />
                  favorites ({favorites.length})
                </button>
                <ul className="contact-list">
                  {favorites.sort().map((room) => (
                    <li key={room} className="contact-item">
                      <a href="#" onClick={(e) => { e.preventDefault(); setSelectedChat({ type: "room", name: room }); }}>
                        <span className="contact-fav-icon">★</span>
                        #{room}
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            )}

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
        </div>}

        {/* right col */}
        <div className="rooms-right">
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
              <input
                className="rooms-search-input"
                type="text"
                placeholder="search rooms…"
                value={roomSearch}
                onChange={(e) => setRoomSearch(e.target.value)}
              />
              <div className="rooms-panel">
                <ul className="rooms-list">
                  {!roomSearch && (
                    <li className="room-item room-item--rules">
                      <Link to="/rules">#rules</Link>
                    </li>
                  )}
                  {Object.entries(grouped).map(([category, rooms]) => {
                    const filtered = roomSearch
                      ? rooms.filter((r) => r.toLowerCase().includes(roomSearch.toLowerCase()))
                      : rooms;
                    if (filtered.length === 0) return null;
                    const isExpanded = !!roomSearch || !collapsed[category];
                    return (
                      <li key={category} className="room-category">
                        <button className="room-category-label" onClick={() => toggleCategory(category)}>
                          <span className={`room-category-arrow ${!isExpanded ? "room-category-arrow--closed" : ""}`} />
                          {category.toLowerCase()}
                        </button>
                        {isExpanded && (
                          <ul className="room-category-list">
                            {filtered.map((room) => (
                              <li key={room} className="room-item">
                                <a href="#" onClick={(e) => { e.preventDefault(); setSelectedChat({ type: "room", name: room }); }}>
                                  #{room}{userCounts[room] > 0 ? ` (${userCounts[room]})` : ""}
                                </a>
                                <button
                                  className={`room-fav-btn${favorites.includes(room) ? " room-fav-btn--active" : ""}`}
                                  title={favorites.includes(room) ? "remove from favorites" : "add to favorites"}
                                  onClick={(e) => { e.preventDefault(); toggleFavorite(room); }}
                                >
                                  {favorites.includes(room) ? "★" : "☆"}
                                </button>
                              </li>
                            ))}
                          </ul>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </div>
            </div>
          )}
        </div>

      </div>

      <div className="rooms-statusbar">
        <span>{connected ? "connected" : "disconnected"}</span>
        <span className="rooms-statusbar-sep">|</span>
        <span>rooms: {Object.values(grouped).flat().length}</span>
        <span className="rooms-statusbar-sep">|</span>
        <span>ping: {ping !== null ? `${ping}ms` : "…"}</span>
      </div>
    </div>
  );
};

export default Rooms;
