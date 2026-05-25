import { MemoryRouter } from "react-router-dom";
import { SmallTalkProvider } from "./context";
import type { SmallTalkProviderProps } from "./context";
import App from "./App";

export interface SmallTalkProps extends Omit<SmallTalkProviderProps, "children"> {
  onClose?: () => void;
  initialX?: number;
  initialY?: number;
}

export function SmallTalk({
  apiUrl,
  wsUrl,
  token,
  username,
  onSignOut,
  onAuthExpired,
  onClose,
  initialX,
  initialY,
}: SmallTalkProps) {
  return (
    <SmallTalkProvider
      apiUrl={apiUrl}
      wsUrl={wsUrl}
      token={token}
      username={username}
      onSignOut={onSignOut}
      onAuthExpired={onAuthExpired}
    >
      <MemoryRouter>
        <App onClose={onClose} initialX={initialX} initialY={initialY} />
      </MemoryRouter>
    </SmallTalkProvider>
  );
}
