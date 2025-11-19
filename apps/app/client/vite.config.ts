import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
  },
  preview: {
    host: "0.0.0.0",
    port: 4173,
    allowedHosts: [
      "localhost",
      "127.0.0.1",
      "ec2-18-175-194-80.eu-west-2.compute.amazonaws.com",
    ],
  },
})
