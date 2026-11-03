import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";

export default defineConfig({
  test: { setupFiles: ["./src/test-setup.ts"] },
  resolve: {
    alias: {
      "@wailsio/runtime": fileURLToPath(new URL("./node_modules/@wailsio/runtime", import.meta.url)),
    },
  },
});
