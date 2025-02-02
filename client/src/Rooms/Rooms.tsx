import React from "react";
import "./Rooms.css";

interface Room {
  name: string;
}

const rooms: Room[] = [
  { name: "cool room 1" },
  { name: "cool room 2" },
  { name: "cool room 3" },
];

const openChatWindow = (roomName: string) => {
  fetch(`/subscribe?room=${roomName}`, { method: "POST" })
    .then(() => {
      window.location.href = `/?room=${roomName}`;
    })
    .catch((error) => console.error("Error:", error));
};

const Rooms: React.FC = () => {
  return (
    <div id="rooms-container">
      <div className="rooms-header">
        <span className="rooms-name">Rooms</span>
      </div>
      <ul className="rooms-list">
        {rooms.map((room, index) => (
          <li key={index} className="room-item">
            <button onClick={() => openChatWindow(room.name)}>{room.name}</button>
          </li>
        ))}
      </ul>
    </div>
  );
};

export default Rooms;
