import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { SmallTalkProvider } from "./context";
import App from "./App.tsx";
import "./index.css";

const root = ReactDOM.createRoot(
  document.getElementById("root") as HTMLElement
);
root.render(
  <SmallTalkProvider
    apiUrl={import.meta.env.VITE_API_HOST ?? "http://localhost:8080"}
    wsUrl={import.meta.env.VITE_WS_HOST ?? "ws://localhost:8080/ws"}
  >
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </SmallTalkProvider>
);
