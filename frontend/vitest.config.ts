import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  define: {
    "import.meta.env.VITE_PRIVACY_POLICY_URL": JSON.stringify(""),
    "import.meta.env.VITE_TERMS_OF_SERVICE_URL": JSON.stringify(""),
    "import.meta.env.VITE_APP_NAME": JSON.stringify("GophDrive"),
    "import.meta.env.VITE_CONTACT_EMAIL": JSON.stringify(""),
    "import.meta.env.VITE_ENCRYPTION_METHOD": JSON.stringify(""),
    "import.meta.env.VITE_JURISDICTION": JSON.stringify(""),
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@docs": path.resolve(__dirname, "../docs"),
    },
  },
});
