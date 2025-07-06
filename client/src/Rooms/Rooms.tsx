import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import "./Rooms.css";
import { API_URL } from "../config";

const Rooms = () => {
  const [rooms, setRooms] = useState<string[]>([]);

  useEffect(() => {
    fetch(`${API_URL}/rooms`)
      .then((response) => response.json())
      .then((data) => {
        const sortedData = data.sort((a: string, b: string) =>
          a.toLowerCase() > b.toLowerCase() ? 1 : -1
        );
        setRooms([...sortedData]);
      })
      .catch((error) => console.error("Fetch error:", error));
  }, []);

  return (
    <div id="rooms-container">
      <div className="rooms-body">
        <div className="welcome">
          <p>Welcome to Small Talk!</p>
          <p>Choose a room to start chatting with others.</p>
          <p>Available rooms:</p>
        </div>
        <ul className="rooms-list">
        {rooms.map((room, index) => (
          <li key={index} className="room-item">
            <Link to={`/${encodeURIComponent(room)}`}>
              {room}
            </Link>
          </li>
        ))}
      </ul>
      </div>
    </div>
  );
};

export default Rooms;
