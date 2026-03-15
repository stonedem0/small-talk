import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import "./Rooms.css";
import { API_URL } from "../config";
import { authFetch } from "../utils/authFetch";

interface RoomsProps {
  unreadDMs?: { [from: string]: number };
  onDMOpen?: (from: string) => void;
}

const Rooms = ({ unreadDMs = {}, onDMOpen }: RoomsProps) => {
  const [grouped, setGrouped] = useState<{ [category: string]: string[] }>({});
  const [collapsed, setCollapsed] = useState<{ [category: string]: boolean }>({});
  const [userCounts, setUserCounts] = useState<{ [room: string]: number }>({});
  const [username, setUsername] = useState<string | null>(null);
  const [dmMessages, setDmMessages] = useState<string[]>([]);
  const [friends, setFriends] = useState<string[]>([]);
  const [friendsCollapsed, setFriendsCollapsed] = useState(false);
  const [dmsCollapsed, setDmsCollapsed] = useState(false);
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
    if (u) setUsername(u);
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

  return (
    <div id="rooms-container">

      {/* top bar */}
      <div className="rooms-topbar">
        <div className="rooms-topbar-avatar">
          <div className="rooms-avatar-dot" />
        </div>
        <div className="rooms-topbar-info">
          <span className="rooms-topbar-name">{username || "User"}</span>
          <span className="rooms-topbar-status">online</span>
        </div>
      </div>

      <div className="rooms-body">

        {/* left col — contacts */}
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
                    <Link to={`/dm/${f}`} onClick={() => onDMOpen?.(f)}>
                      <span className="contact-status-dot contact-status-dot--online" />
                      {f}
                    </Link>
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
                      <Link to={`/dm/${partner}`} onClick={() => onDMOpen?.(partner)}>
                        <span className="contact-status-dot" />
                        {partner}
                        {unreadDMs[partner] ? <span className="contact-unread">{unreadDMs[partner]}</span> : null}
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>

        {/* right col — rooms */}
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
                        <Link to={`/${encodeURIComponent(room)}`}>
                          #{room}{userCounts[room] > 0 ? ` (${userCounts[room]})` : ""}
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        </div>
        </div>

      </div>
    </div>
  );
};

export default Rooms;
