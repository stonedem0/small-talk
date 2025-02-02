import React, { useEffect, useState } from "react";
import "./Rooms.css";

interface Room {
  name: string;
}

// const rooms: Room[] = [
//   { name: "cool room 1" },
//   { name: "cool room 2" },
//   { name: "cool room 3" },
// ];

const openChatWindow = (roomName: string) => {
  fetch(`/subscribe?room=${roomName}`, { method: "POST" })
    .then(() => {
      //   window.location.href = `/?room=${roomName}`;
      console.log("🚪 Opening chat window for room:", roomName);
    })
    .catch((error) => console.error("Error:", error));
};

const Rooms: React.FC = () => {
  //   console.log("🚪 Opening chat window for room:");
  const [rooms, setRooms] = useState<string[]>([]);
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
    <div id="rooms-container">
      <div className="rooms-header">
        <span className="rooms-name">Rooms</span>
      </div>
      <ul className="rooms-list">
        {rooms.map(
          (room, index) => (
            console.log("🚪 Opening chat window for room:", room),
            (
              <li key={index} className="room-item">
                <button
                  onClick={() => {
                    console.log("🚪 Opening chat window for room:", room);
                    openChatWindow(room);
                  }}
                >
                  {room}
                </button>
              </li>
            )
          )
        )}
      </ul>
    </div>
  );
};

export default Rooms;
