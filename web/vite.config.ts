import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/watch": "http://localhost:8080",
      // The branding preview is a server-rendered page the settings iframe
      // loads; without this the dev server answers it with the SPA shell.
      "/branding": "http://localhost:8080",
    },
  },
});
