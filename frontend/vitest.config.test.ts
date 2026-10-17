// @vitest-environment node
import { fileURLToPath } from "node:url";
import { mkdtemp, mkdir, copyFile, writeFile, symlink, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, join } from "node:path";
import { createVitest } from "vitest/node";
import { describe, expect, it } from "vitest";
import { isFileLoadingAllowed, resolveConfig } from "vite";

const root = fileURLToPath(new URL(".", import.meta.url));
const config = (name: string) => resolveConfig({
  root,
  configFile: resolve(root, name),
  envDir: false,
}, "serve");

describe("hidden-worktree filesystem policy", () => {
  it.each(["unit{one,two}", "unit[one]"])("denies Git metadata with a literal %s worktree path", async (name) => {
    const scratch = await mkdtemp(join(tmpdir(), "vitest-hidden-root-"));
    let runner: Awaited<ReturnType<typeof createVitest>> | undefined;
    try {
      const isolatedRoot = join(scratch, ".git", "odd-worktrees", name, "frontend");
      await mkdir(join(isolatedRoot, "nested", ".git"), { recursive: true });
      await copyFile(resolve(root, "vitest.config.ts"), join(isolatedRoot, "vitest.config.ts"));
      await copyFile(resolve(root, "vite.config.ts"), join(isolatedRoot, "vite.config.ts"));
      await writeFile(join(isolatedRoot, "package.json"), '{"type":"module"}');
      await symlink(resolve(root, "node_modules"), join(isolatedRoot, "node_modules"), "dir");
      await writeFile(join(isolatedRoot, "nested", ".git", "config"), "FAKE_GIT_SECRET");
      runner = await createVitest({ root: isolatedRoot, config: join(isolatedRoot, "vitest.config.ts"), watch: false }, {
        envDir: false,
        cacheDir: join(scratch, "cache"),
        server: { watch: null },
      });
      const allowed = [runner.vite.config, ...runner.projects.map((project) => project.vite.config)]
        .map((config) => isFileLoadingAllowed(config, join(isolatedRoot, "nested", ".git", "config")));
      let result: unknown;
      try {
        result = await runner.projects[0].vite.transformRequest("/nested/.git/config?raw");
      } catch (error) {
        result = { code: (error as { code?: string }).code };
      }
      expect({ allowed, result }).toEqual({ allowed: allowed.map(() => false), result: { code: "ERR_DENIED_ID" } });
      expect(JSON.stringify(result)).not.toContain("FAKE_GIT_SECRET");
    } finally {
      await runner?.close();
      await rm(scratch, { recursive: true, force: true });
    }
  });

  it("allows test sources without exposing sensitive files or Git metadata", async () => {
    const test = await config("vitest.config.ts");
    expect(test.server.fs.strict).toBe(true);
    expect(isFileLoadingAllowed(test, resolve(root, "src/App.tsx"))).toBe(true);
    expect(isFileLoadingAllowed(test, resolve(root, "src/wails-service.ts"))).toBe(true);

    // These are hypothetical paths: never read real credentials or Git metadata.
    for (const name of [
      ".env", ".env.local", "nested/.env.production",
      "private.key", "private.pem", "private.crt", "private.p12",
      "private.pfx", "private.cer", "private.der",
      ".npmrc", ".yarnrc.yml", "nested/.npmrc", "nested/.yarnrc.yml",
      ".git", ".git/config", "nested/.git", "nested/.git/objects/example",
    ]) {
      expect(isFileLoadingAllowed(test, resolve(root, name)), name).toBe(false);
    }
    if (root.split("/").includes(".git")) {
      expect(test.server.fs.allow).toEqual([root.replace(/\/$/, "")]);
      for (const name of ["../.git", "../src/example.ts", "../../../config", "../../../objects/example"]) {
        expect(isFileLoadingAllowed(test, resolve(root, name)), name).toBe(false);
      }
    }
  });

  it("preserves installed sensitive-file defaults and the development Git deny", async () => {
    const defaults = await resolveConfig({ configFile: false, envDir: false }, "serve");
    const dev = await config("vite.config.ts");
    const test = await config("vitest.config.ts");
    expect(dev.server.fs.strict).toBe(true);
    expect(dev.server.fs.deny).toEqual(defaults.server.fs.deny);
    expect(dev.server.fs.deny).toContain("**/.git/**");
    for (const pattern of defaults.server.fs.deny.filter((p) => p !== "**/.git/**")) {
      expect(test.server.fs.deny).toContain(pattern);
    }
    expect(test.resolve.alias).toEqual(dev.resolve.alias);
    if (root.split("/").includes(".git")) {
      expect(isFileLoadingAllowed(dev, resolve(root, "src/App.tsx"))).toBe(false);
    }
  });
});
