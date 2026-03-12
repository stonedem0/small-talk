import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import "./Rooms.css";
import { API_URL } from "../config";
import { authFetch } from "../utils/authFetch";

const Rooms = () => {
  const [grouped, setGrouped] = useState<{ [category: string]: string[] }>({});
  const [collapsed, setCollapsed] = useState<{ [category: string]: boolean }>({});
  const [userCounts, setUserCounts] = useState<{ [room: string]: number }>({});
  const [username, setUsername] = useState<string | null>(null);
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
        // only set collapsed for new categories; preserve existing state
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
    const username = localStorage.getItem("username");
    if (username) {
      setUsername(username);
    }
  }, []);
  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) return;
    fetchRooms();

    // Fetch online user counts
    authFetch(`${API_URL}/online-users`)
      .then((response) => response.json())
      .then((data) => setUserCounts(data))
      .catch((error) => {
        console.error("online-users fetch error", error);
      });
  }, []);

  return (
    <div id="rooms-container">
      <div className="rooms-body">
        <div className="welcome">
        <span className="username">
                  oh hai, <strong>{username || "User"}</strong>!
                </span>
          <p>glad you're here 👋</p>
          <p><strong>small talk</strong> is a pet project built to learn distributed systems through something actually fun - real-time chat with high intensity data flow.</p>
          <p>designed &amp; built by me. yes, all of it. yes, it works.</p>
          <p>pick a room and say hi - check <strong>#rules</strong> first. break them and i will ban your ip, no warnings.</p>
          <p>love, stonedemo 💜</p>
        </div>
        <div className="rooms-sidebar">
          <div className="rooms-scroll-container">
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
