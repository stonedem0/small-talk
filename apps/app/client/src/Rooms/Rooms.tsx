import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import "./Rooms.css";
import { API_URL } from "../config";

const Rooms = () => {
  const [rooms, setRooms] = useState<string[]>([]);
  const [userCounts, setUserCounts] = useState<{ [room: string]: number }>({});
  const [username, setUsername] = useState<string | null>(null);
  useEffect(() => {
    const username = localStorage.getItem("username");
    if (username) {
      setUsername(username);
    }
  }, []);
  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      // No token found
      // navigate("/login");
      return;
    }
    fetch(`${API_URL}/rooms`, {
      headers: {
        "Authorization": `Bearer ${localStorage.getItem("token")}`
      }
    })
      .then((response) => response.json())
      .then((data) => {
        const sortedData = data.sort((a: string, b: string) =>
          a.toLowerCase() > b.toLowerCase() ? 1 : -1
        );

        const allRooms = [...new Set([...sortedData].sort((a, b) =>
          a.toLowerCase() > b.toLowerCase() ? 1 : -1
        ))];
        setRooms(allRooms);
      })
      .catch((error) => {
        console.error("rooms list fetch error", error);
      });

    // Fetch online user counts
    fetch(`${API_URL}/online-users`, {
      headers: {
        "Authorization": `Bearer ${localStorage.getItem("token")}`
      }
    })
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
        <div className="rooms-scroll-container">
          <ul className="rooms-list">
            <li className="room-item room-item--rules">
              <Link to="/rules">#rules</Link>
            </li>
          {rooms.map((room, index) => (
            <li key={index} className="room-item">
              <Link to={`/${encodeURIComponent(room)}`}>
                {room}{userCounts[room] > 0 ? ` (${userCounts[room]})` : ""}
              </Link>
            </li>
          ))}
          </ul>
        </div>
      </div>
    </div>
  );
};

export default Rooms;
