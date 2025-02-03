import React, { useEffect, useState } from "react";
import "./Rooms.css";
import Chat from "../Chat/Chat";

const Rooms: React.FC = () => {
  const [rooms, setRooms] = useState<string[]>([]);
  const [selectedRoom, setSelectedRoom] = useState<string>("");

  useEffect(() => {
    fetch("http://localhost:8080/rooms")
      .then((response) => response.json())
      .then((data) => {
        console.log("Setting rooms:", data);
        setRooms([...data]); // Ensure a new reference is created
      })
      .catch((error) => console.error("Fetch error:", error));
  }, []);

  return (
    <div id="chat-app">
      <Chat roomName={selectedRoom} username="Anonymous" />
      <div id="rooms-container">
        <div className="rooms-header">
          <div className="rooms-title">
            <span className="rooms-icon"></span>
            <span className="rooms-name">rooms</span>
          </div>
        </div>
        <ul className="rooms-list">
          {rooms.map((room, index) => (
            <li key={index} className="room-item">
              <button
                onClick={() => {
                  console.log("🚪 Opening chat window for room:", room);
                  setSelectedRoom(room);
                }}
              >
                {room}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};

export default Rooms;
