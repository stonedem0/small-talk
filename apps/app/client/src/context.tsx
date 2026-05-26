import { createContext, useContext, useState, useEffect, useRef } from "react";
import type { MutableRefObject, ReactNode } from "react";

const PREFIX = "st:";

export const storage = {
  get: (key: string) => localStorage.getItem(PREFIX + key),
  set: (key: string, value: string) => localStorage.setItem(PREFIX + key, value),
  remove: (key: string) => localStorage.removeItem(PREFIX + key),
};

// Module-level ref so authFetch (not a hook) can call authExpired without window.dispatchEvent
export const authExpiredCallback: { current: (() => void) | null } = { current: null };
// Module-level refs so non-hook code (authFetch, etc.) can access the configured URLs
export const apiUrlRef: { current: string } = { current: "" };
export const wsUrlRef: { current: string } = { current: "" };

export interface SmallTalkContextValue {
  token: string | null;
  username: string | null;
  apiUrl: string;
  wsUrl: string;
  setToken: (token: string | null) => void;
  setUsername: (username: string | null) => void;
  signOut: () => void;
  authExpired: () => void;
  wsRef: MutableRefObject<WebSocket | null>;
  roomsRevision: number;
  bumpRoomsRevision: () => void;
}

export const SmallTalkContext = createContext<SmallTalkContextValue | null>(null);

export function useSmallTalk(): SmallTalkContextValue {
  const ctx = useContext(SmallTalkContext);
  if (!ctx) throw new Error("useSmallTalk must be used within SmallTalkProvider");
  return ctx;
}

export interface SmallTalkProviderProps {
  apiUrl: string;
  wsUrl: string;
  token?: string | null;
  username?: string | null;
  onSignOut?: () => void;
  onAuthExpired?: () => void;
  children: ReactNode;
}

export function SmallTalkProvider({
  apiUrl,
  wsUrl,
  token: tokenProp,
  username: usernameProp,
  onSignOut,
  onAuthExpired,
  children,
}: SmallTalkProviderProps) {
  apiUrlRef.current = apiUrl;
  wsUrlRef.current = wsUrl;

  const [token, setTokenState] = useState<string | null>(
    tokenProp ?? storage.get("token")
  );
  const [username, setUsernameState] = useState<string | null>(
    usernameProp ?? storage.get("username")
  );
  const [roomsRevision, setRoomsRevision] = useState(0);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (tokenProp !== undefined) setTokenState(tokenProp);
  }, [tokenProp]);

  useEffect(() => {
    if (usernameProp !== undefined) setUsernameState(usernameProp);
  }, [usernameProp]);

  const setToken = (t: string | null) => {
    setTokenState(t);
    if (t) storage.set("token", t);
    else storage.remove("token");
  };

  const setUsername = (u: string | null) => {
    setUsernameState(u);
    if (u) storage.set("username", u);
    else storage.remove("username");
  };

  const signOut = () => {
    setToken(null);
    setUsername(null);
    storage.remove("dm_notifications");
    storage.remove("rooms_selected_chat");
    storage.remove("rooms_contacts_hidden");
    onSignOut?.();
  };

  const authExpired = () => {
    setToken(null);
    setUsername(null);
    onAuthExpired?.();
  };

  const bumpRoomsRevision = () => setRoomsRevision((n) => n + 1);

  // Keep module-level ref in sync so authFetch can call it without hooks
  useEffect(() => {
    authExpiredCallback.current = authExpired;
  });

  return (
    <SmallTalkContext.Provider
      value={{
        token, username, apiUrl, wsUrl,
        setToken, setUsername, signOut, authExpired,
        wsRef, roomsRevision, bumpRoomsRevision,
      }}
    >
      {children}
    </SmallTalkContext.Provider>
  );
}
