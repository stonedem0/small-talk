import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const previewAllowedHosts =
  (
    process.env.VITE_PREVIEW_ALLOWED_HOSTS
      ?.split(",")
      .map((h) => h.trim())
      .filter(Boolean)
  ) ?? ["localhost", "127.0.0.1"];

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
  },
  preview: {
    host: "0.0.0.0",
    port: 4173,
    allowedHosts: previewAllowedHosts,
  },
});
