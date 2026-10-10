/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_API_HOST: string;
    readonly VITE_WS_HOST: string;
    readonly VITE_DIRECTORY_URL?: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}

interface Window {
    /** The open chat socket, read by the window menu to announce a rename. */
    currentWebSocket?: WebSocket;
}
