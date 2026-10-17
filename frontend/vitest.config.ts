import { fileURLToPath } from "node:url";
import { resolveConfig, normalizePath, mergeConfig } from "vite";
import { defineConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default defineConfig(async () => {
  const root = normalizePath(fileURLToPath(new URL(".", import.meta.url))).replace(/\/$/, "");
  const defaults = await resolveConfig({ configFile: false, envDir: false }, "serve");
  // Only a hidden worktree needs the ancestor-directory exception.
  if (!root.split("/").includes(".git")) return viteConfig;

  // The filesystem path is literal, not glob syntax (e.g. unit{one,two}).
  const literalRoot = root.replace(/[\\*?[\]{}()!+@|^$]/g, "\\$&");
  return mergeConfig(viteConfig, {
    server: {
      fs: {
        strict: true,
        allow: [root],
        deny: [
          ...defaults.server.fs.deny.filter((pattern) => pattern !== "**/.git/**"),
          "**/.git", // Includes a linked worktree's pointer file.
          `${literalRoot}/**/.git/**`, // Nested Git metadata is still private.
        ],
      },
    },
  });
});
