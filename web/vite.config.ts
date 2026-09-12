import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The API base URL is baked in at build time via VITE_API_BASE_URL (see
// docker-compose.yaml's web service) rather than configured at runtime —
// this is a static SPA with no server-side component of its own, so
// there's no request-time place to inject it. Defaults to the gateway's
// local dev port for `npm run dev` without docker compose.
export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
});
