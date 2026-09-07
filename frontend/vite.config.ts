import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";

export default defineConfig({
  resolve: {
    alias: {
      "@wailsio/runtime": fileURLToPath(new URL("./node_modules/@wailsio/runtime", import.meta.url)),
    },
  },
});
