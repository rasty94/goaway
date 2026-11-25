import path from "path";
import { defineConfig } from "vitest/config";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [
    react(),
    // plugin-react v6 dropped its `babel` option; the compiler runs through rolldown's babel plugin.
    babel({ presets: [reactCompilerPreset()] }),
    tailwindcss()
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src")
    }
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/test/**", "src/main.tsx", "src/vite-env.d.ts"],
      reporter: ["text-summary"],
      // ponytail: floor, not target; raise it as component tests grow.
      thresholds: { lines: 60 }
    }
  },
  build: {
    assetsDir: "assets",
    sourcemap: false,
    cssCodeSplit: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            if (id.includes("react") || id.includes("react-dom"))
              return "react-vendor";
            if (id.includes("motion")) return "motion";
            if (id.includes("@phosphor-icons/react")) return "icons";
            return "vendor";
          }
          if (id.includes("/src/pages/")) return "pages";
          if (id.includes("/src/components/")) return "components";
          return null;
        }
      }
    }
  }
});
