import React, { useEffect, useState } from "react";
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
      console.log("No token found");
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
        console.error("Fetch error:", error);
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
        console.error("User count fetch error:", error);
      });
  }, []);

  return (
    <div id="rooms-container">
      <div className="rooms-body">
        <div className="welcome">
        <span className="username">
                  oh hai, <strong>{username || "User"}</strong>!
                </span>
          <p>Welcome to Small Talk!</p>
          <p>Choose a room to start chatting with others.</p>
          <p>Available rooms:</p>
          <p>
            Small Talk is a place to connect, share, and have fun conversations on any topic you like.<br />
            Select a room from the list to join a discussion, or just browse to see what others are talking about.<br />
            Don't see a topic you like? Feel free to suggest a new room to the community!
          </p>
        </div>
        <div className="rooms-scroll-container">
          <ul className="rooms-list">
          {rooms.map((room, index) => (
            <li key={index} className="room-item">
              <Link to={`/${encodeURIComponent(room)}`}>
                {room}{userCounts[room] > 0 ? ` (${userCounts[room]})` : ""}
              </Link>
            </li>
          ))}
          </ul>
          {/* <button  className="create-room-button" onClick={async () => {
            const roomName = prompt("Enter room name:");
            if (!roomName) {
              return;
            }
            const response = await fetch(`${API_URL}/create-room`, {
              method: "POST",
              headers: {
                "Authorization": `Bearer ${localStorage.getItem("token")}`
              },
              body: JSON.stringify({ room: roomName })
            });
            if (!response.ok) {
              alert(await response.text());
              return;
            }
            alert("Room created successfully");
            window.location.reload();
          }}>Create Room</button> */}
        </div>
      </div>
    </div>
  );
};

export default Rooms;
