"use client";

import * as React from "react";
import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";

export function ThemeToggle() {
  const { setTheme, theme } = useTheme();

  return (
    <div style={{ display: "flex", gap: "0.5rem" }}>
      <button
        onClick={() => setTheme("light")}
        className={`btn ${theme === "light" ? "btn-primary" : ""}`}
        aria-label="Light mode"
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          padding: "0.5rem",
        }}
      >
        <Sun size={18} />
      </button>
      <button
        onClick={() => setTheme("dark")}
        className={`btn ${theme === "dark" ? "btn-primary" : ""}`}
        aria-label="Dark mode"
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          padding: "0.5rem",
        }}
      >
        <Moon size={18} />
      </button>
      <button
        onClick={() => setTheme("system")}
        className={`btn ${theme === "system" ? "btn-primary" : ""}`}
        aria-label="System mode"
        style={{ fontSize: "0.8rem", padding: "0.5rem 1rem" }}
      >
        System
      </button>
    </div>
  );
}
