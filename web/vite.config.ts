import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// The Go server renders its own index.html (ui/templates/index.html) that
// loads /ui/<version>/app.js and /ui/<version>/app.css, so the bundle must
// have fixed file names and no hashed chunks.
export default defineConfig({
  plugins: [react()],
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      output: {
        entryFileNames: "app.js",
        chunkFileNames: "chunk-[name].js",
        assetFileNames: (info) => {
          const name = info.names?.[0] ?? "";
          return name.endsWith(".css") ? "app.css" : "assets/[name][extname]";
        },
        inlineDynamicImports: true,
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      // `bun dev` talks to a bot running locally on :8080
      "/api": {
        target: process.env.NINJABOT_API ?? "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
