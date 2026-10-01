import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // public/.gitkeep is copied into dist on every build: the Go app embeds
  // dist, which must exist (and be versioned) before the first build.
  publicDir: "public",
  test: {
    include: ["src/**/*.test.ts"],
  },
});
