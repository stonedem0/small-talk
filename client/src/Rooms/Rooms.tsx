import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./Rooms.css";
import { API_URL } from "../config";

interface RoomsProps {
  username: string;
}

const Rooms: React.FC<RoomsProps> = () => {
  const [rooms, setRooms] = useState<string[]>([]);
  const navigate = useNavigate();

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
      <div className="rooms-header">
        <div className="rooms-title">
          <span className="rooms-icon"></span>
          <span className="rooms-name">Rooms</span>
        </div>
      </div>
      <ul className="rooms-list">
        {rooms.map((room, index) => (
          <li key={index} className="room-item">
            <button
              onClick={() => {
                console.log("🚪 Navigating to chat room:", room);
                navigate(`/${encodeURIComponent(room)}`);
              }}
            >
              {room}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
};

export default Rooms;
