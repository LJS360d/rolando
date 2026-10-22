import { defineConfig } from "astro/config";
import preact from "@astrojs/preact";
import tailwindcss from "@tailwindcss/vite";
import sitemap from '@astrojs/sitemap';

try { process.loadEnvFile(); } catch {}
const env = process.env;

export default defineConfig({
  site: env.SITE_URL || "http://localhost:4321",
  integrations: [preact(), sitemap({
    filter: (page) => !/\/(admin)\/?$/.test(page),
  })],
  build: { format: "file" },
  server: { port: Number(env.PORT || 4321) },
  vite: {
    plugins: [tailwindcss()],
    server: { proxy: { "/api": env.API_URL || "http://localhost:8090" } },
  },
});
