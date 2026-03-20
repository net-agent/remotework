import { defineConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default defineConfig(async () => {
  const resolved = await viteConfig({ command: "serve", mode: "test" });

  return {
    ...resolved,
    test: {
      environment: "node",
      include: ["src/**/*.{test,spec}.{ts,tsx}"],
    },
  };
});
