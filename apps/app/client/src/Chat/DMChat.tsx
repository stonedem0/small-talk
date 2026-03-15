import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { API_URL } from "../config";
import { authFetch } from "../utils/authFetch";
import Chat from "./Chat";

interface DMChatProps {
  username: string;
  targetUsernameOverride?: string;
}

const DMChat = ({ username, targetUsernameOverride }: DMChatProps) => {
  const { targetUsername: targetUsernameParam } = useParams<{ targetUsername: string }>();
  const targetUsername = targetUsernameOverride ?? targetUsernameParam;
  const navigate = useNavigate();
  const [room, setRoom] = useState<string | null>(null);

  useEffect(() => {
    if (!targetUsername) { navigate("/"); return; }

    authFetch(`${API_URL}/dm/start`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${localStorage.getItem("token")}`,
      },
      body: JSON.stringify({ target: targetUsername }),
    }).then(async (res) => {
      if (!res.ok) { navigate("/"); return; }
      const { room } = await res.json();
      setRoom(room);
    }).catch(() => navigate("/"));
  }, [targetUsername, navigate]);

  if (!room) return null;

  return <Chat username={username} roomNameOverride={room} />;
};

export default DMChat;
